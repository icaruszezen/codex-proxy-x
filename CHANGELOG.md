# Changelog

## [1.15.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.14.1...v1.15.0) (2026-05-21)


### ✨ 新功能

* add support for image generation response handling ([8fea398](https://github.com/icaruszezen/codex-proxy-x/commit/8fea39888f546d830de82345ae8a05c3dacc7655))
* **auth:** implement account enable/disable functionality ([932681c](https://github.com/icaruszezen/codex-proxy-x/commit/932681c3c4e40d7426d12b318f25f4b33006fcd4))
* **stats:** implement bulk recovery feature for accounts ([0f0afec](https://github.com/icaruszezen/codex-proxy-x/commit/0f0afec665b2c518fce9d25f647a96639d87d7cc))
* 修复特定情况下的换号逻辑错误问题 ([622d08d](https://github.com/icaruszezen/codex-proxy-x/commit/622d08d31b740488c953f968f755551b02b1c3be))
* 实现非流式请求的 Codex SSE 收集功能，优化响应处理逻辑，支持更高效的错误处理和账号使用记录 ([55cf46a](https://github.com/icaruszezen/codex-proxy-x/commit/55cf46a81c18b770ccbbd89971f564126583d47c))
* 实验性支持Image模型，支持命令行OAuth登录授权Codex账号 ([40a9bfd](https://github.com/icaruszezen/codex-proxy-x/commit/40a9bfd722b254fea8e90000eeb759aaaf932fff))
* 支持 sub2api 导出文件格式的多账号 JSON 导入，增强解析功能 ([9376650](https://github.com/icaruszezen/codex-proxy-x/commit/93766500b14b52a55b8f1fc16b42126c3f428233))
* 更新内置版本号，新增5.5模型版本枚举 ([0f6e25a](https://github.com/icaruszezen/codex-proxy-x/commit/0f6e25a9bd06e248b3a44bf0fc4b9cad0f2aeca6))
* 添加 qmsg 私聊通知功能，支持配置和测试消息推送 ([489c636](https://github.com/icaruszezen/codex-proxy-x/commit/489c636d24c43176dd939f2e40826e2944a9a0e6))
* 添加刷新禁用功能，更新数据库结构和相关逻辑以支持账号的刷新状态管理 ([74b8d34](https://github.com/icaruszezen/codex-proxy-x/commit/74b8d34810202438bfe8453d1dbbcb65784da6d3))
* 添加账号硬删除功能，支持通过 API 删除本地账号及持久化存储，更新相关文档和前端逻辑 ([d60e201](https://github.com/icaruszezen/codex-proxy-x/commit/d60e2019570b5463508e17997d2e2fb124b6da50))


### 🐛 错误修复

* stream ([7cb6ae2](https://github.com/icaruszezen/codex-proxy-x/commit/7cb6ae2bb7533e57d136b1148717d380e271f385))
* stream ([1d534c0](https://github.com/icaruszezen/codex-proxy-x/commit/1d534c0400c0873bce85cb28db19a1c89f54837c))
* 非流502 ([9183c20](https://github.com/icaruszezen/codex-proxy-x/commit/9183c206fba63f729c2958d736382c7f2034e6f5))
* 非流502 ([24dce68](https://github.com/icaruszezen/codex-proxy-x/commit/24dce6885d32cf4750e21ff2ed85b2df5982ca07))


### 🎨 代码样式

* 优化界面布局 ([b543bbb](https://github.com/icaruszezen/codex-proxy-x/commit/b543bbbdfb86568fa466b61488ebd5b8d2195da0))


### 🔧 其他更新

* **master:** release 1.10.0 ([89ae1c2](https://github.com/icaruszezen/codex-proxy-x/commit/89ae1c23049911f8768d7949c19b6516c660b016))
* **master:** release 1.10.0 ([2ce17a3](https://github.com/icaruszezen/codex-proxy-x/commit/2ce17a3e769fc4b68214784fb73e2762e59f0932))
* **master:** release 1.10.1 ([4ff0ee7](https://github.com/icaruszezen/codex-proxy-x/commit/4ff0ee7c3ec72502d0673f17b690ff90c942f9da))
* **master:** release 1.10.1 ([b8ac820](https://github.com/icaruszezen/codex-proxy-x/commit/b8ac820303ce638ec58b9b4ccd45b92a74fdcec4))
* **master:** release 1.11.0 ([5ad88bc](https://github.com/icaruszezen/codex-proxy-x/commit/5ad88bcc4f03dc8ef30256c96012b5c15835c7b2))
* **master:** release 1.11.0 ([ffe9792](https://github.com/icaruszezen/codex-proxy-x/commit/ffe9792c70ee4a0349308a97f82c7a7bcc21a83e))
* **master:** release 1.12.0 ([8b15686](https://github.com/icaruszezen/codex-proxy-x/commit/8b15686afc2cd3f7bf1c22c02362c692e2f33c65))
* **master:** release 1.12.0 ([0bbb21d](https://github.com/icaruszezen/codex-proxy-x/commit/0bbb21d87ba356728d83ebaed55b1dec54b6f460))
* **master:** release 1.13.0 ([7b354cc](https://github.com/icaruszezen/codex-proxy-x/commit/7b354cc3ec9f1a6da6fea53aee8c6d07d520feca))
* **master:** release 1.13.0 ([6f1a6a8](https://github.com/icaruszezen/codex-proxy-x/commit/6f1a6a8eecda9c7568aa9eb227bc4dad820f1039))
* **master:** release 1.14.0 ([87c18aa](https://github.com/icaruszezen/codex-proxy-x/commit/87c18aa371789f8b43fc8673b8970d657051d14b))
* **master:** release 1.14.0 ([9b54016](https://github.com/icaruszezen/codex-proxy-x/commit/9b540168e4bac9ae414bc583b8e5869f33b1099f))
* **master:** release 1.14.1 ([535c88a](https://github.com/icaruszezen/codex-proxy-x/commit/535c88a9e0941b1f1a0c848f10f5e7afd3bfe289))
* **master:** release 1.7.0 ([#34](https://github.com/icaruszezen/codex-proxy-x/issues/34)) ([490a2f6](https://github.com/icaruszezen/codex-proxy-x/commit/490a2f6bb71c994f1e8aa69e550e122e6ec1accf))
* **master:** release 1.8.0 ([7256754](https://github.com/icaruszezen/codex-proxy-x/commit/72567549df23fa0695d0535bd04dec893404b324))
* **master:** release 1.8.0 ([1069607](https://github.com/icaruszezen/codex-proxy-x/commit/106960742eb8526db48fc90dab1ac4a39f79d40a))
* **master:** release 1.8.0 ([#35](https://github.com/icaruszezen/codex-proxy-x/issues/35)) ([ef8675f](https://github.com/icaruszezen/codex-proxy-x/commit/ef8675f5a2cb20f9f41ead5fc9970e313a912464))
* **master:** release 1.8.1 ([#38](https://github.com/icaruszezen/codex-proxy-x/issues/38)) ([20dd626](https://github.com/icaruszezen/codex-proxy-x/commit/20dd626a8789d11e7fc0ba831d365ca83b0d2899))
* **master:** release 1.9.0 ([7c95862](https://github.com/icaruszezen/codex-proxy-x/commit/7c95862121db5c72a0bc79de469baec6d6daf7d2))
* **master:** release 1.9.0 ([1dc0032](https://github.com/icaruszezen/codex-proxy-x/commit/1dc0032613295895a90cd259bee47f38dbeaecfb))


### 📦 依赖更新

* **ci:** bump googleapis/release-please-action from 4 to 5 ([#37](https://github.com/icaruszezen/codex-proxy-x/issues/37)) ([7d20366](https://github.com/icaruszezen/codex-proxy-x/commit/7d20366110655c1f4bdce39f78fc7c309ad75b5f))
* **go:** bump modernc.org/sqlite from 1.49.1 to 1.50.0 ([#36](https://github.com/icaruszezen/codex-proxy-x/issues/36)) ([e6b1aee](https://github.com/icaruszezen/codex-proxy-x/commit/e6b1aee4464f286d5516bc4c3d0482c386d87315))

## [1.14.1](https://github.com/icaruszezen/codex-proxy-x/compare/v1.14.0...v1.14.1) (2026-05-21)


### 🐛 错误修复

* 设置默认图片请求路径的模型为 gpt-5.5-image ([534a618](https://github.com/icaruszezen/codex-proxy-x/commit/534a6180f59c24d4a9001e249d4f884228cd05d3))
* 支持/v1/images/generations ([a829d44](https://github.com/icaruszezen/codex-proxy-x/commit/a829d44796737e34dc88a86ec397c6aa6ee65859))

## [1.14.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.13.0...v1.14.0) (2026-05-19)


### ✨ 新功能

* 实现非流式请求的 Codex SSE 收集功能，优化响应处理逻辑，支持更高效的错误处理和账号使用记录 ([55cf46a](https://github.com/icaruszezen/codex-proxy-x/commit/55cf46a81c18b770ccbbd89971f564126583d47c))
* 添加账号硬删除功能，支持通过 API 删除本地账号及持久化存储，更新相关文档和前端逻辑 ([d60e201](https://github.com/icaruszezen/codex-proxy-x/commit/d60e2019570b5463508e17997d2e2fb124b6da50))


### 🎨 代码样式

* 优化界面布局 ([b543bbb](https://github.com/icaruszezen/codex-proxy-x/commit/b543bbbdfb86568fa466b61488ebd5b8d2195da0))

## [1.13.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.12.0...v1.13.0) (2026-05-10)


### ✨ 新功能

* 添加刷新禁用功能，更新数据库结构和相关逻辑以支持账号的刷新状态管理 ([74b8d34](https://github.com/icaruszezen/codex-proxy-x/commit/74b8d34810202438bfe8453d1dbbcb65784da6d3))

## [1.12.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.11.0...v1.12.0) (2026-05-10)


### ✨ 新功能

* 支持 sub2api 导出文件格式的多账号 JSON 导入，增强解析功能 ([9376650](https://github.com/icaruszezen/codex-proxy-x/commit/93766500b14b52a55b8f1fc16b42126c3f428233))
* 添加 qmsg 私聊通知功能，支持配置和测试消息推送 ([489c636](https://github.com/icaruszezen/codex-proxy-x/commit/489c636d24c43176dd939f2e40826e2944a9a0e6))

## [1.11.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.10.1...v1.11.0) (2026-05-05)


### ✨ 新功能

* 修复特定情况下的换号逻辑错误问题 ([622d08d](https://github.com/icaruszezen/codex-proxy-x/commit/622d08d31b740488c953f968f755551b02b1c3be))

## [1.10.1](https://github.com/icaruszezen/codex-proxy-x/compare/v1.10.0...v1.10.1) (2026-05-01)


### 🐛 错误修复

* stream ([7cb6ae2](https://github.com/icaruszezen/codex-proxy-x/commit/7cb6ae2bb7533e57d136b1148717d380e271f385))
* 非流502 ([9183c20](https://github.com/icaruszezen/codex-proxy-x/commit/9183c206fba63f729c2958d736382c7f2034e6f5))

## [1.10.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.9.0...v1.10.0) (2026-04-29)


### ✨ 新功能

* **auth:** implement account enable/disable functionality ([932681c](https://github.com/icaruszezen/codex-proxy-x/commit/932681c3c4e40d7426d12b318f25f4b33006fcd4))

## [1.9.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.8.0...v1.9.0) (2026-04-27)


### ✨ 新功能

* add support for image generation response handling ([8fea398](https://github.com/icaruszezen/codex-proxy-x/commit/8fea39888f546d830de82345ae8a05c3dacc7655))

## [1.8.0](https://github.com/icaruszezen/codex-proxy-x/compare/v1.7.0...v1.8.0) (2026-04-24)


### ✨ 新功能

* **proxy:** 添加 /v1/responses/compact 路由和测试 ([e997591](https://github.com/icaruszezen/codex-proxy-x/commit/e997591e76cd28ace393394964c2f6c7ae42864f))
* **static:** add asset handling and serve static files ([00340e4](https://github.com/icaruszezen/codex-proxy-x/commit/00340e4973f56039f80d8b1ea953752193fcfb28))
* **stats:** implement bulk recovery feature for accounts ([0f0afec](https://github.com/icaruszezen/codex-proxy-x/commit/0f0afec665b2c518fce9d25f647a96639d87d7cc))
* 修复部分情况下的账号请求429问题，优化换号与请求实现，完善配置文件相关示例 ([0a59ad9](https://github.com/icaruszezen/codex-proxy-x/commit/0a59ad9de8257c854a42a868e4d094920a07b9c4))
* 实验性支持Image模型，支持命令行OAuth登录授权Codex账号 ([40a9bfd](https://github.com/icaruszezen/codex-proxy-x/commit/40a9bfd722b254fea8e90000eeb759aaaf932fff))
* 支持自适应连接池配置，优化请求与转发性能 ([b73d9a1](https://github.com/icaruszezen/codex-proxy-x/commit/b73d9a10b3a5325bbed85df4630ac5ee906f3339))
* 添加可选是否显示1m and fast ([53fddea](https://github.com/icaruszezen/codex-proxy-x/commit/53fddeabf7504fda2ad21d72a1e377981af7dcd0))
* 适配1m模型，修复fast模型参数传递问题，性能优化 ([3906118](https://github.com/icaruszezen/codex-proxy-x/commit/3906118993315ee813f919e96c14b40dfc5fe3ac))


### 🐛 错误修复

* 优化工作流打包文件错误问题 ([2381ddb](https://github.com/icaruszezen/codex-proxy-x/commit/2381ddb4deec2405e82a6b94c3339ada23954b3f))
* 修复1m上下文与fast模式参数传递错误问题，细节优化 ([d601552](https://github.com/icaruszezen/codex-proxy-x/commit/d60155275d1ef2d68eb6b26e2e1b2b239a23cd73))
* 修复在auth文件为空时的panic问题，支持rk为空或null的支持 ([4214ee9](https://github.com/icaruszezen/codex-proxy-x/commit/4214ee9b6d30657cdb26d84ae5c4d75d433dbfbe))
* 修复工作流配置权限错误问题 ([1ed95c9](https://github.com/icaruszezen/codex-proxy-x/commit/1ed95c9459cb751b079c7b5ac3feccefd8f800d4))
* 删除测试文件 ([bea00da](https://github.com/icaruszezen/codex-proxy-x/commit/bea00da3a567691a8313487901350bab084ae3c7))


### 🔧 其他更新

* **master:** release 1.2.0 ([#12](https://github.com/icaruszezen/codex-proxy-x/issues/12)) ([6ca53c6](https://github.com/icaruszezen/codex-proxy-x/commit/6ca53c639fd82bb2ec8724f46fe6c972bae9620c))
* **master:** release 1.2.1 ([#17](https://github.com/icaruszezen/codex-proxy-x/issues/17)) ([82424f5](https://github.com/icaruszezen/codex-proxy-x/commit/82424f558e3234f5780c998f94272e9555eec094))
* **master:** release 1.3.0 ([#18](https://github.com/icaruszezen/codex-proxy-x/issues/18)) ([1d2a820](https://github.com/icaruszezen/codex-proxy-x/commit/1d2a8205739e55b98b6d79ceaeea31d7b50c5123))
* **master:** release 1.4.0 ([#19](https://github.com/icaruszezen/codex-proxy-x/issues/19)) ([eeae522](https://github.com/icaruszezen/codex-proxy-x/commit/eeae5223907c218e483352910952b405068e6967))
* **master:** release 1.5.0 ([#20](https://github.com/icaruszezen/codex-proxy-x/issues/20)) ([cfa9dbb](https://github.com/icaruszezen/codex-proxy-x/commit/cfa9dbbd89eb742e4b14130c1a81c577179b1256))
* **master:** release 1.6.0 ([#25](https://github.com/icaruszezen/codex-proxy-x/issues/25)) ([a3dabe7](https://github.com/icaruszezen/codex-proxy-x/commit/a3dabe7d0d91452508204ffb0daf1acb7e2a422a))
* **master:** release 1.6.1 ([#31](https://github.com/icaruszezen/codex-proxy-x/issues/31)) ([1d22afd](https://github.com/icaruszezen/codex-proxy-x/commit/1d22afd272e6e3c9d2690fc9a918a1b6fbd12df1))
* **master:** release 1.6.2 ([#32](https://github.com/icaruszezen/codex-proxy-x/issues/32)) ([54773de](https://github.com/icaruszezen/codex-proxy-x/commit/54773deed5e5f0543cadbb43fe50fce7b0f5387c))
* **master:** release 1.7.0 ([#34](https://github.com/icaruszezen/codex-proxy-x/issues/34)) ([490a2f6](https://github.com/icaruszezen/codex-proxy-x/commit/490a2f6bb71c994f1e8aa69e550e122e6ec1accf))
* update repository references from XxxXTeam to icaruszezen ([95a4a5d](https://github.com/icaruszezen/codex-proxy-x/commit/95a4a5d46786f2c1ca46730f770c714bcd457f0f))


### 📦 依赖更新

* **ci:** bump actions/checkout from 4 to 6 ([#14](https://github.com/icaruszezen/codex-proxy-x/issues/14)) ([ed47be0](https://github.com/icaruszezen/codex-proxy-x/commit/ed47be0f38666a716f7a83973748886b0303a7a2))
* **ci:** bump actions/download-artifact from 4 to 8 ([#23](https://github.com/icaruszezen/codex-proxy-x/issues/23)) ([37ed431](https://github.com/icaruszezen/codex-proxy-x/commit/37ed43151b998b035991412f681c1d5b88b2839a))
* **ci:** bump actions/setup-go from 5 to 6 ([#15](https://github.com/icaruszezen/codex-proxy-x/issues/15)) ([3dda587](https://github.com/icaruszezen/codex-proxy-x/commit/3dda5878976d19aa7b9f443f33909773cdae21e1))
* **ci:** bump actions/upload-artifact from 4 to 7 ([#24](https://github.com/icaruszezen/codex-proxy-x/issues/24)) ([eda64e7](https://github.com/icaruszezen/codex-proxy-x/commit/eda64e7c14f277700c892715bb24eea796498c74))
* **ci:** bump softprops/action-gh-release from 2 to 3 ([#30](https://github.com/icaruszezen/codex-proxy-x/issues/30)) ([b5d0bd1](https://github.com/icaruszezen/codex-proxy-x/commit/b5d0bd19f52acac22a28a4deea1289643232019a))
* **go:** bump github.com/lib/pq from 1.12.0 to 1.12.3 ([#22](https://github.com/icaruszezen/codex-proxy-x/issues/22)) ([25e876e](https://github.com/icaruszezen/codex-proxy-x/commit/25e876e30f94a63d79d5a97bc089296130dd6850))
* **go:** bump github.com/valyala/fasthttp from 1.69.0 to 1.70.0 ([#29](https://github.com/icaruszezen/codex-proxy-x/issues/29)) ([abadc64](https://github.com/icaruszezen/codex-proxy-x/commit/abadc64a705bdf6562063f4ea5bf33ae76264890))
* **go:** bump golang.org/x/net in the golang-org-x group ([#27](https://github.com/icaruszezen/codex-proxy-x/issues/27)) ([90bf8e7](https://github.com/icaruszezen/codex-proxy-x/commit/90bf8e71dbfefad47229e86b3040159904f6a406))
* **go:** bump modernc.org/sqlite from 1.47.0 to 1.48.0 ([#13](https://github.com/icaruszezen/codex-proxy-x/issues/13)) ([955d498](https://github.com/icaruszezen/codex-proxy-x/commit/955d4987eac279f08ac0002fdc40c59791d34acd))
* **go:** bump modernc.org/sqlite from 1.48.0 to 1.48.1 ([#21](https://github.com/icaruszezen/codex-proxy-x/issues/21)) ([a86e8ef](https://github.com/icaruszezen/codex-proxy-x/commit/a86e8eff0eebf5a50e9fbe68dd19997f983f31c2))
* **go:** bump modernc.org/sqlite from 1.48.1 to 1.48.2 ([#28](https://github.com/icaruszezen/codex-proxy-x/issues/28)) ([2956c27](https://github.com/icaruszezen/codex-proxy-x/commit/2956c273e1f8109e830ef884b9a88a55181f885c))
* **go:** bump modernc.org/sqlite from 1.48.2 to 1.49.1 ([#33](https://github.com/icaruszezen/codex-proxy-x/issues/33)) ([dffa40f](https://github.com/icaruszezen/codex-proxy-x/commit/dffa40f6f075afea410bc3e919b0254847687f51))


### 🎡 持续集成

* 优化工作流 ([c13a796](https://github.com/icaruszezen/codex-proxy-x/commit/c13a796aaa344d6a590d7cbd5805991a3a2132b4))
* 修复工作流 ([f5b4e6b](https://github.com/icaruszezen/codex-proxy-x/commit/f5b4e6b562d11ce33729350b81c93fe3e824a6a5))
* 修复自动发版 ([524dd12](https://github.com/icaruszezen/codex-proxy-x/commit/524dd12858dbcc0eb32ce3f6bbd571f7ee780745))
* 分支写错了... ([88e19dc](https://github.com/icaruszezen/codex-proxy-x/commit/88e19dc33137ba4f88aa158b9912664645f26424))
* 更新依赖版本以及go.sum文件 ([1507119](https://github.com/icaruszezen/codex-proxy-x/commit/15071197f2d1b93c182920a109284fe69b026f40))
* 添加自动发版 ([815b727](https://github.com/icaruszezen/codex-proxy-x/commit/815b7271b09c4b8cfdae4a593b3635a7862ffde3))

## [1.7.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.6.2...v1.7.0) (2026-04-22)


### ✨ 新功能

* 实验性支持Image模型，支持命令行OAuth登录授权Codex账号 ([40a9bfd](https://github.com/XxxXTeam/codex-proxy/commit/40a9bfd722b254fea8e90000eeb759aaaf932fff))


### 📦 依赖更新

* **go:** bump modernc.org/sqlite from 1.48.2 to 1.49.1 ([#33](https://github.com/XxxXTeam/codex-proxy/issues/33)) ([dffa40f](https://github.com/XxxXTeam/codex-proxy/commit/dffa40f6f075afea410bc3e919b0254847687f51))

## [1.6.2](https://github.com/XxxXTeam/codex-proxy/compare/v1.6.1...v1.6.2) (2026-04-16)


### 🐛 错误修复

* 修复1m上下文与fast模式参数传递错误问题，细节优化 ([d601552](https://github.com/XxxXTeam/codex-proxy/commit/d60155275d1ef2d68eb6b26e2e1b2b239a23cd73))

## [1.6.1](https://github.com/XxxXTeam/codex-proxy/compare/v1.6.0...v1.6.1) (2026-04-14)


### 📦 依赖更新

* **ci:** bump softprops/action-gh-release from 2 to 3 ([#30](https://github.com/XxxXTeam/codex-proxy/issues/30)) ([b5d0bd1](https://github.com/XxxXTeam/codex-proxy/commit/b5d0bd19f52acac22a28a4deea1289643232019a))
* **go:** bump github.com/valyala/fasthttp from 1.69.0 to 1.70.0 ([#29](https://github.com/XxxXTeam/codex-proxy/issues/29)) ([abadc64](https://github.com/XxxXTeam/codex-proxy/commit/abadc64a705bdf6562063f4ea5bf33ae76264890))
* **go:** bump golang.org/x/net in the golang-org-x group ([#27](https://github.com/XxxXTeam/codex-proxy/issues/27)) ([90bf8e7](https://github.com/XxxXTeam/codex-proxy/commit/90bf8e71dbfefad47229e86b3040159904f6a406))
* **go:** bump modernc.org/sqlite from 1.48.1 to 1.48.2 ([#28](https://github.com/XxxXTeam/codex-proxy/issues/28)) ([2956c27](https://github.com/XxxXTeam/codex-proxy/commit/2956c273e1f8109e830ef884b9a88a55181f885c))


### 🎡 持续集成

* 更新依赖版本以及go.sum文件 ([1507119](https://github.com/XxxXTeam/codex-proxy/commit/15071197f2d1b93c182920a109284fe69b026f40))

## [1.6.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.5.0...v1.6.0) (2026-04-11)


### ✨ 新功能

* 修复部分情况下的账号请求429问题，优化换号与请求实现，完善配置文件相关示例 ([0a59ad9](https://github.com/XxxXTeam/codex-proxy/commit/0a59ad9de8257c854a42a868e4d094920a07b9c4))
* 添加可选是否显示1m and fast ([53fddea](https://github.com/XxxXTeam/codex-proxy/commit/53fddeabf7504fda2ad21d72a1e377981af7dcd0))


### 📦 依赖更新

* **ci:** bump actions/download-artifact from 4 to 8 ([#23](https://github.com/XxxXTeam/codex-proxy/issues/23)) ([37ed431](https://github.com/XxxXTeam/codex-proxy/commit/37ed43151b998b035991412f681c1d5b88b2839a))
* **ci:** bump actions/upload-artifact from 4 to 7 ([#24](https://github.com/XxxXTeam/codex-proxy/issues/24)) ([eda64e7](https://github.com/XxxXTeam/codex-proxy/commit/eda64e7c14f277700c892715bb24eea796498c74))
* **go:** bump github.com/lib/pq from 1.12.0 to 1.12.3 ([#22](https://github.com/XxxXTeam/codex-proxy/issues/22)) ([25e876e](https://github.com/XxxXTeam/codex-proxy/commit/25e876e30f94a63d79d5a97bc089296130dd6850))
* **go:** bump modernc.org/sqlite from 1.48.0 to 1.48.1 ([#21](https://github.com/XxxXTeam/codex-proxy/issues/21)) ([a86e8ef](https://github.com/XxxXTeam/codex-proxy/commit/a86e8eff0eebf5a50e9fbe68dd19997f983f31c2))

## [1.5.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.4.0...v1.5.0) (2026-04-02)


### ✨ 新功能

* **proxy:** 添加 /v1/responses/compact 路由和测试 ([e997591](https://github.com/XxxXTeam/codex-proxy/commit/e997591e76cd28ace393394964c2f6c7ae42864f))


### 🐛 错误修复

* 删除测试文件 ([bea00da](https://github.com/XxxXTeam/codex-proxy/commit/bea00da3a567691a8313487901350bab084ae3c7))

## [1.4.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.3.0...v1.4.0) (2026-03-31)


### ✨ 新功能

* 支持自适应连接池配置，优化请求与转发性能 ([b73d9a1](https://github.com/XxxXTeam/codex-proxy/commit/b73d9a10b3a5325bbed85df4630ac5ee906f3339))


### 🐛 错误修复

* 优化工作流打包文件错误问题 ([2381ddb](https://github.com/XxxXTeam/codex-proxy/commit/2381ddb4deec2405e82a6b94c3339ada23954b3f))
* 修复工作流配置权限错误问题 ([1ed95c9](https://github.com/XxxXTeam/codex-proxy/commit/1ed95c9459cb751b079c7b5ac3feccefd8f800d4))

## [1.3.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.2.1...v1.3.0) (2026-03-31)


### ✨ 新功能

* 适配1m模型，修复fast模型参数传递问题，性能优化 ([3906118](https://github.com/XxxXTeam/codex-proxy/commit/3906118993315ee813f919e96c14b40dfc5fe3ac))

## [1.2.1](https://github.com/XxxXTeam/codex-proxy/compare/v1.2.0...v1.2.1) (2026-03-30)


### 🐛 错误修复

* 修复在auth文件为空时的panic问题，支持rk为空或null的支持 ([4214ee9](https://github.com/XxxXTeam/codex-proxy/commit/4214ee9b6d30657cdb26d84ae5c4d75d433dbfbe))


### 📦 依赖更新

* **ci:** bump actions/checkout from 4 to 6 ([#14](https://github.com/XxxXTeam/codex-proxy/issues/14)) ([ed47be0](https://github.com/XxxXTeam/codex-proxy/commit/ed47be0f38666a716f7a83973748886b0303a7a2))
* **ci:** bump actions/setup-go from 5 to 6 ([#15](https://github.com/XxxXTeam/codex-proxy/issues/15)) ([3dda587](https://github.com/XxxXTeam/codex-proxy/commit/3dda5878976d19aa7b9f443f33909773cdae21e1))
* **go:** bump modernc.org/sqlite from 1.47.0 to 1.48.0 ([#13](https://github.com/XxxXTeam/codex-proxy/issues/13)) ([955d498](https://github.com/XxxXTeam/codex-proxy/commit/955d4987eac279f08ac0002fdc40c59791d34acd))

## [1.2.0](https://github.com/XxxXTeam/codex-proxy/compare/v1.1.5...v1.2.0) (2026-03-27)


### ✨ 新功能

* pump-phase upstream retry, ExecuteNonStream scanner error handling, and openCodexResponsesBody struct return ([#10](https://github.com/XxxXTeam/codex-proxy/issues/10)) ([7bed650](https://github.com/XxxXTeam/codex-proxy/commit/7bed650a919d98d952125b0bf3dea5712390c0ae))
* retry on upstream error during stream pump; fix ExecuteNonStream scanner error handling ([93d5560](https://github.com/XxxXTeam/codex-proxy/commit/93d55602eb72c3d7be3d725962c76510d2c21939))


### 🐛 错误修复

* 修复部分情况没有走代理的bug,添加代理启动时检查 ([c1322e2](https://github.com/XxxXTeam/codex-proxy/commit/c1322e2517654ed669e5d6d5240831d4536e77f9))


### ♻️ 代码重构

* group openCodexResponsesBody returns into struct and remove unused context params ([c19603f](https://github.com/XxxXTeam/codex-proxy/commit/c19603fcc2c90156e49f1986d9b12bdc84b6f724))


### 📦 依赖更新

* **go:** bump filippo.io/edwards25519 from 1.1.0 to 1.1.1 ([#8](https://github.com/XxxXTeam/codex-proxy/issues/8)) ([b3552db](https://github.com/XxxXTeam/codex-proxy/commit/b3552db88f9d5d5ec5668c500197c63e78d6b41c))


### 🎡 持续集成

* 优化工作流 ([c13a796](https://github.com/XxxXTeam/codex-proxy/commit/c13a796aaa344d6a590d7cbd5805991a3a2132b4))
* 修复工作流 ([f5b4e6b](https://github.com/XxxXTeam/codex-proxy/commit/f5b4e6b562d11ce33729350b81c93fe3e824a6a5))
* 修复自动发版 ([524dd12](https://github.com/XxxXTeam/codex-proxy/commit/524dd12858dbcc0eb32ce3f6bbd571f7ee780745))
* 分支写错了... ([88e19dc](https://github.com/XxxXTeam/codex-proxy/commit/88e19dc33137ba4f88aa158b9912664645f26424))
* 添加自动发版 ([815b727](https://github.com/XxxXTeam/codex-proxy/commit/815b7271b09c4b8cfdae4a593b3635a7862ffde3))
