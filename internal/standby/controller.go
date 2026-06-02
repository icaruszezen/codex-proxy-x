/**
 * 备用账号池协调器
 *
 * 在主 Manager 与备用 Manager 之间做选号路由：
 *   1. 每次选号先尝试主池，主池可选 → 顺便停用备用模式；
 *   2. 主池无可选账号且备用池有可用账号 → CAS 激活备用模式 + qmsg 通知 + 启动备用池 Token 刷新和额度冷却复查；
 *   3. 主池一旦再能选号 → CAS 停用备用模式 + qmsg 通知 + 取消备用池激活期后台任务。
 *
 * 未激活时备用池实例 *不会* 被周期性刷新/健康检查触发，账号保持"纯净"。
 */
package standby

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"codex-proxy/internal/auth"
	"codex-proxy/internal/notify"

	log "github.com/sirupsen/logrus"
)

/**
 * Controller 主备账号池协调器
 *
 * @field primary - 主账号池 Manager
 * @field standby - 备用账号池 Manager（DB 模式按 is_standby=1 过滤）
 * @field quotaChecker - 额度检查器（用于 Manager 内部 401/429 恢复链路）
 * @field qmsgService - qmsg 通知服务（激活/停用通知）
 * @field active - 是否处于备用模式（true=正在使用备用池）
 * @field activeCancel - 备用池激活期后台任务的 cancel；停用时调用以退出
 * @field rootCtx - 父 context；激活生命周期均派生自此
 */
type Controller struct {
	primary       *auth.Manager
	standby       *auth.Manager
	quotaChecker  *auth.QuotaChecker
	qmsgService   *notify.Service
	newapiService *notify.NewAPIService

	active       atomic.Bool
	primaryDown  atomic.Bool
	activeMu     sync.Mutex
	activeCancel context.CancelFunc
	rootCtx      atomic.Pointer[context.Context]
}

/**
 * StateSnapshot 对外展示的协调器状态快照（供 /admin/standby/state 使用）
 */
type StateSnapshot struct {
	Active       bool   `json:"active"`
	StandbyTotal int    `json:"standby_total"`
	PrimaryTotal int    `json:"primary_total"`
	Note         string `json:"note,omitempty"`
}

/**
 * New 创建 Controller 实例
 *
 * @param primary - 主池 Manager（必填）
 * @param standby - 备用池 Manager（必填；可以无账号）
 * @param qc - 额度检查器（可空）
 * @param qmsgService - qmsg 通知服务（可空，则不发通知）
 * @returns *Controller
 */
func New(primary, standby *auth.Manager, qc *auth.QuotaChecker, qmsgService *notify.Service) *Controller {
	if primary == nil || standby == nil {
		return nil
	}
	c := &Controller{
		primary:      primary,
		standby:      standby,
		quotaChecker: qc,
		qmsgService:  qmsgService,
	}
	bg := context.Background()
	c.rootCtx.Store(&bg)
	return c
}

/**
 * Start 绑定根 context；停用时仅取消备用池刷新 goroutine，不影响 manager 全局生命周期
 * @param ctx - 进程级 context
 */
func (c *Controller) Start(ctx context.Context) {
	if c == nil || ctx == nil {
		return
	}
	c.rootCtx.Store(&ctx)
}

/**
 * SetNewAPIService 绑定 NewAPI 渠道控制服务；未配置时自动启停静默跳过
 */
func (c *Controller) SetNewAPIService(service *notify.NewAPIService) {
	if c == nil {
		return
	}
	c.newapiService = service
}

/**
 * Primary 返回主池 Manager
 */
func (c *Controller) Primary() *auth.Manager {
	if c == nil {
		return nil
	}
	return c.primary
}

/**
 * Standby 返回备用池 Manager
 */
func (c *Controller) Standby() *auth.Manager {
	if c == nil {
		return nil
	}
	return c.standby
}

/**
 * IsActive 是否当前处于备用模式
 */
func (c *Controller) IsActive() bool {
	if c == nil {
		return false
	}
	return c.active.Load()
}

/**
 * ManagerOf 根据账号判断所属 Manager（账号路径以备用池实例化时的目录为前缀或 DB 前缀）
 * 找不到时返回 nil
 */
func (c *Controller) ManagerOf(acc *auth.Account) *auth.Manager {
	if c == nil || acc == nil {
		return nil
	}
	if c.primary != nil && c.primary.AccountInPool(acc) {
		return c.primary
	}
	if c.standby != nil && c.standby.AccountInPool(acc) {
		return c.standby
	}
	return nil
}

func (c *Controller) PickPrimaryOnly(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.primary == nil {
		return nil, fmt.Errorf("standby controller 未初始化")
	}
	acc, err := c.primary.PickExcluding(model, excluded)
	if err != nil {
		c.markPrimaryUnavailableForNewAPI()
		return nil, err
	}
	c.markPrimaryAvailableForNewAPI()
	c.deactivateIfActive()
	return acc, nil
}

/**
 * Pick 主入口：先尝试主池 → 成功则停用备用模式；否则尝试激活备用池并选号
 *
 * @param model - 模型名（透传给 selector）
 * @param excluded - 本次请求中已经尝试过且被排除的账号 FilePath 集合
 * @returns *auth.Account - 选中账号
 * @returns error - 主备都无可用账号时返回最后的错误
 */
func (c *Controller) Pick(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.primary == nil {
		return nil, fmt.Errorf("standby controller 未初始化")
	}
	if acc, err := c.primary.PickExcluding(model, excluded); err == nil {
		c.markPrimaryAvailableForNewAPI()
		c.deactivateIfActive()
		return acc, nil
	} else {
		c.markPrimaryUnavailableForNewAPI()
		if c.standby == nil || c.standby.AccountCount() == 0 {
			return nil, err
		}
		if acc2, err2 := c.standby.PickExcluding(model, excluded); err2 == nil {
			c.activateIfNeeded()
			return acc2, nil
		} else {
			/* 备用池也无可选，返回主池错误更具语义 */
			return nil, err
		}
	}
}

func (c *Controller) PickStandbyOnly(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.standby == nil || c.standby.AccountCount() == 0 {
		return nil, fmt.Errorf("备用账号池无可用账号")
	}
	if c.primary != nil {
		if acc, err := c.primary.PickExcluding(model, excluded); err == nil {
			c.markPrimaryAvailableForNewAPI()
			c.deactivateIfActive()
			return acc, nil
		} else {
			c.markPrimaryUnavailableForNewAPI()
		}
	}
	acc, err := c.standby.PickExcluding(model, excluded)
	if err != nil {
		return nil, err
	}
	c.activateIfNeeded()
	return acc, nil
}

func (c *Controller) PickStandbyRecentlySuccessful(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.standby == nil || c.standby.AccountCount() == 0 {
		return nil, fmt.Errorf("备用账号池无可用账号")
	}
	if c.primary != nil {
		if acc, err := c.primary.PickRecentlySuccessful(model, excluded); err == nil {
			c.markPrimaryAvailableForNewAPI()
			c.deactivateIfActive()
			return acc, nil
		} else {
			c.markPrimaryUnavailableForNewAPI()
		}
	}
	acc, err := c.standby.PickRecentlySuccessful(model, excluded)
	if err != nil {
		return nil, err
	}
	c.activateIfNeeded()
	return acc, nil
}

func (c *Controller) PickStandbyIgnoringCooldown(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.standby == nil || c.standby.AccountCount() == 0 {
		return nil, fmt.Errorf("备用账号池无可用账号")
	}
	if c.primary != nil {
		if acc, err := c.primary.PickIgnoringCooldown(model, excluded); err == nil {
			c.markPrimaryAvailableForNewAPI()
			c.deactivateIfActive()
			return acc, nil
		} else {
			c.markPrimaryUnavailableForNewAPI()
		}
	}
	acc, err := c.standby.PickIgnoringCooldown(model, excluded)
	if err != nil {
		return nil, err
	}
	c.activateIfNeeded()
	return acc, nil
}

/**
 * PickRecentlySuccessful fallback 路径：跟随当前 active 状态选池
 */
func (c *Controller) PickRecentlySuccessful(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.primary == nil {
		return nil, fmt.Errorf("standby controller 未初始化")
	}
	if c.active.Load() && c.standby != nil && c.standby.AccountCount() > 0 {
		if acc, err := c.standby.PickRecentlySuccessful(model, excluded); err == nil {
			return acc, nil
		}
	}
	return c.primary.PickRecentlySuccessful(model, excluded)
}

/**
 * PickIgnoringCooldown 429 并发重试场景：跟随当前 active 状态选池
 */
func (c *Controller) PickIgnoringCooldown(model string, excluded map[string]bool) (*auth.Account, error) {
	if c == nil || c.primary == nil {
		return nil, fmt.Errorf("standby controller 未初始化")
	}
	if c.active.Load() && c.standby != nil && c.standby.AccountCount() > 0 {
		if acc, err := c.standby.PickIgnoringCooldown(model, excluded); err == nil {
			return acc, nil
		}
	}
	return c.primary.PickIgnoringCooldown(model, excluded)
}

/**
 * SyncPrimaryAvailabilityForNewAPI 根据主池当前可选号状态同步 NewAPI 渠道状态。
 * 管理端手动启停/删除/导入账号时调用，弥补无代理请求时不会进入 Pick 的场景。
 */
func (c *Controller) SyncPrimaryAvailabilityForNewAPI() {
	if c == nil || c.primary == nil {
		return
	}
	if c.primary.HasPickableAccount() {
		c.markPrimaryAvailableForNewAPI()
	} else {
		c.markPrimaryUnavailableForNewAPI()
	}
}

/**
 * EnsureTokenFresh 按账号所属池路由到正确 Manager 执行
 */
func (c *Controller) EnsureTokenFresh(ctx context.Context, acc *auth.Account) bool {
	mgr := c.ManagerOf(acc)
	if mgr == nil {
		return false
	}
	return mgr.EnsureTokenFresh(ctx, acc)
}

/**
 * Snapshot 返回当前状态快照
 */
func (c *Controller) Snapshot() StateSnapshot {
	if c == nil {
		return StateSnapshot{}
	}
	snap := StateSnapshot{Active: c.active.Load()}
	if c.primary != nil {
		snap.PrimaryTotal = c.primary.AccountCount()
	}
	if c.standby != nil {
		snap.StandbyTotal = c.standby.AccountCount()
	}
	if snap.Active {
		snap.Note = "正在使用备用账号池"
	} else if snap.StandbyTotal == 0 {
		snap.Note = "备用账号池未配置账号"
	} else {
		snap.Note = "主池可用，备用池待机"
	}
	return snap
}

/**
 * activateIfNeeded CAS 激活备用模式：发 qmsg 通知，启动备用池激活期后台任务
 * 重复激活无副作用（CAS 失败直接返回）
 */
func (c *Controller) activateIfNeeded() {
	if !c.active.CompareAndSwap(false, true) {
		return
	}
	standbyTotal := 0
	if c.standby != nil {
		standbyTotal = c.standby.AccountCount()
	}
	log.Warnf("备用账号池已启用: 主池无可用账号，已切换至备用池（%d 个账号）", standbyTotal)

	c.activeMu.Lock()
	if c.activeCancel == nil && c.standby != nil {
		rootPtr := c.rootCtx.Load()
		root := context.Background()
		if rootPtr != nil {
			root = *rootPtr
		}
		ctx, cancel := context.WithCancel(root)
		c.activeCancel = cancel
		go func() {
			defer log.Info("备用账号池 Token 刷新循环已退出")
			c.standby.StartRefreshLoop(ctx)
		}()
		if c.quotaChecker != nil {
			go func() {
				defer log.Info("备用账号池额度冷却复查循环已退出")
				c.standby.StartQuotaCooldownRecheckLoop(ctx, c.quotaChecker)
			}()
		}
	}
	c.activeMu.Unlock()

	c.sendQmsg("[备用账号池] 主池已无可用账号，已自动启用备用账号池（%d 个账号）", standbyTotal)
}

/**
 * deactivateIfActive CAS 停用备用模式：发 qmsg 通知，取消备用池激活期后台任务
 */
func (c *Controller) deactivateIfActive() {
	if !c.active.CompareAndSwap(true, false) {
		return
	}
	primaryTotal := 0
	if c.primary != nil {
		primaryTotal = c.primary.AccountCount()
	}
	log.Infof("备用账号池已停用: 主池已恢复（%d 个账号）", primaryTotal)

	c.activeMu.Lock()
	if c.activeCancel != nil {
		c.activeCancel()
		c.activeCancel = nil
	}
	c.activeMu.Unlock()

	c.sendQmsg("[备用账号池] 主账号池已恢复，已自动停用备用账号池（主池当前 %d 个账号）", primaryTotal)
}

/**
 * sendQmsg 异步发送 qmsg 通知；未配置/未启用时静默
 */
func (c *Controller) sendQmsg(format string, args ...any) {
	if c == nil || c.qmsgService == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.qmsgService.Send(ctx, msg); err != nil && err != notify.ErrQmsgDisabled {
			log.Warnf("备用账号池 qmsg 通知发送失败: %v", err)
		}
	}()
}

func (c *Controller) markPrimaryUnavailableForNewAPI() {
	if !c.newAPIAutoSwitchEnabled() {
		return
	}
	if !c.primaryDown.CompareAndSwap(false, true) {
		return
	}
	c.sendNewAPIDisable()
}

func (c *Controller) markPrimaryAvailableForNewAPI() {
	if !c.newAPIAutoSwitchEnabled() {
		return
	}
	if !c.primaryDown.CompareAndSwap(true, false) {
		return
	}
	c.sendNewAPIEnable()
}

func (c *Controller) newAPIAutoSwitchEnabled() bool {
	if c == nil || c.newapiService == nil {
		return false
	}
	return c.newapiService.Config().AutoSwitch
}

func (c *Controller) sendNewAPIDisable() {
	if c == nil || c.newapiService == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.newapiService.DisableChannel(ctx); err != nil && err != notify.ErrNewAPIAutoSwitchDisabled {
			log.Warnf("备用账号池启用时禁用 NewAPI 渠道失败: %v", err)
		}
	}()
}

func (c *Controller) sendNewAPIEnable() {
	if c == nil || c.newapiService == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := c.newapiService.EnableChannel(ctx); err != nil && err != notify.ErrNewAPIAutoSwitchDisabled {
			log.Warnf("备用账号池停用时启用 NewAPI 渠道失败: %v", err)
		}
	}()
}
