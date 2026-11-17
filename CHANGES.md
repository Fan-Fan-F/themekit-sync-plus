# 变更日志 CHANGES — themekit-sync-plus

## 一、定位复核与结论

- **核对原仓库**：[Shopify/themekit](https://github.com/Shopify/themekit)
- **许可证**：MIT（已核对根目录 LICENSE 文件，原作者 Shopify 版权声明完整保留）。
- **原项目现状**：
  - 仓库状态：未归档（`archived: false`，`disabled: false`），Star 数 1327。
  - 维护现状：Shopify 官方已标记为 Deprecated，建议普通主题开发者在单店交互式开发中转向基于 Node.js/OAuth 的 Shopify CLI；但官方至今仍在维护关键安全修复（如近期合并的 PR #1046 路径校验补丁）。
- **与 Shopify CLI 的关系及二次开发价值**：
  - Shopify CLI 偏重单店本地热重载交互开发，体积庞大（需完整 Node.js/Ruby 运行时及 npm 依赖），强依赖浏览器 OAuth 交互鉴权，在无头 CI/CD 流水线（GitHub Actions、GitLab CI、Docker 容器化构建）以及跨境多店铺矩阵批量部署中十分笨重且难以无交互运行。
  - ThemeKit 采用 Go 语言编写，为轻量级单一静态二进制，零运行时依赖，基于直接 API Token/Password 与 `config.yml` 配置，在自动化部署、批处理与无头环境中具备不可替代的优势。
  - 原项目痛点：在多店铺同步能力、大主题目录扫描性能（缺乏本地校验缓存、全量扫描内存占用高）、并发控制与 `--dry-run` 预览支持上存在明显欠缺，且存在若干文件操作与多环境处理的已知 bug。
- **定位结论**：**定位成立**。面向出海/跨境多店铺矩阵与轻量 CI/CD 自动化流水线，二次开发增强版本具备明确的刚需生态位与实用价值。

---

## 二、实质改进计划

1. **大主题目录高性能增量同步与本地校验缓存机制（Incremental Sync Cache）**
   - **痛点**：原版 `deploy` 时通过 `shopify.FindAssets` 遍历全量文件并对每个文件执行全量读取与 base64 编码，海量资源（成百上千个图片、字体、脚本）导致大量内存分配与重复 I/O。
   - **改进**：引入基于文件 `mtime + size` 的本地增量校验缓存（`.themekit_cache.json`），未变动文件直接复用校验和，跳过无谓的重读与编码；支持 `--no-cache` 绕过缓存。

2. **多环境/多店铺批量同步增强（Multi-Store & Multi-Environment Sync Enhancement）**
   - **痛点**：原版 `--env` (`-e`) 仅支持单一环境或通配符，不支持逗号分隔指定多个店铺（如 `-e us-store,eu-store` 会被当作单一不存在的环境名报错）；多店铺部署时遇单店网络/凭证故障会导致全局提前退出，缺乏错误隔离与汇总报表。
   - **改进**：支持逗号分隔多店铺参数解析、环境去重与确定性排序；增加 `--continue-on-error` 容错选项，支持单店失败时继续部署其余店铺，并在结尾输出多店铺同步状态汇总报表。

3. **并发工作池与限流保护机制（Configurable Concurrency Worker Pool）**
   - **痛点**：原版在 `deploy` 和 `download` 时直接为全量文件（如数千个文件）瞬间并发启动数千个 goroutine，竞争全局信号量，极易导致连接争用、内存尖峰及 Shopify API 突发限流。
   - **改进**：使用受控的 Worker Pool 替代无界 goroutine 倾泻，新增 `--concurrency` 参数（默认 10，支持 1-50 调节），平滑多店铺与大工程的并发压力。

4. **安全预览模式（Dry-Run Mode）**
   - **痛点**：在多店铺生产环境中执行 `deploy` 或清理时，若配置不当容易误删或误覆盖线上主题，原版缺乏无破坏性的预检机制。
   - **改进**：全局增加 `--dry-run` 参数，模拟执行比对计算，输出拟更新、删除、跳过的操作明细，而不向 Shopify 发出任何写/删请求，方便 CI/CD 预检与审计。

5. **已知缺陷修复与文件操作鲁棒性增强（Bug Fixes & Robustness）**
   - **修复 1**：修复 `src/shopify/asset.go` 中 `loadAssetsFromDirectory` 静默忽略 `ReadAsset` 错误的 bug（原代码 `var asset, _ = ReadAsset(e, assetKey) // TODO handle error`），防止将空资产结构体带入同步流程。
   - **修复 2**：重构 `cmd/deploy.go` 中 `settings_data.json` 的延后处理机制，摒弃易受异常流程影响的 `defer perform`，在所有普通资产并发同步完成后显式有序执行并正确捕获错误。
   - **修复 3**：优化 `.gitignore`，将本地缓存文件与编译产物纳入忽略，杜绝临时文件污染仓库。

---

## 三、代码改动记录

1. **`src/shopify/cache.go` & `src/shopify/cache_test.go`**（新增）：
   - 实现 `ChecksumCache` 结构，提供 `Get`, `Set`, `Save`, `Invalidate`, `Count` 等方法。
   - 记录 `mtime_nano + size -> checksum` 映射关系，文件无修改时直接命中缓存，避免昂贵的磁盘重新读取与全量哈希计算。
2. **`src/shopify/asset.go`**：
   - 新增 `FindAssetsWithCache` 与 `readAssetWithCache`，支持携带缓存进行资产快速扫描。
   - 修复 `loadAssetsFromDirectory` 中原本静默忽略错误的 `// TODO handle error` 坏味道，改为正确向调用链返回错误。
3. **`src/cmdutil/util.go` & `src/cmdutil/util_test.go`**：
   - `Flags` 结构体扩充 `NoCache`, `DryRun`, `Concurrency`, `ContinueOnError` 4 个核心配置项。
   - 改造 `expandEnvironments`：支持逗号分隔多店铺定义（如 `-e "store-us, store-eu"`）、环境去重与确定性排序，并补充单元测试。
   - 改造 `forEachClient`：支持 `--continue-on-error` 错误隔离模式，在多店铺同步结束时输出结构化汇总报表（Multi-Environment Sync Summary）。
4. **`cmd/theme.go`**：
   - 注册 `--no-cache`, `--dry-run`, `--concurrency`, `--continue-on-error` 全局持久化命令行参数。
5. **`cmd/deploy.go` & `cmd/download.go`**：
   - 引入受控 Worker Pool 并发执行模型，替代瞬间倾泻数千个 goroutine 的旧设计。
   - `deploy` 显式后置同步 `config/settings_data.json`，确保原子性与错误捕获。
   - 支持 `--dry-run` 预览日志提示。
6. **`cmd/watch.go`**：
   - `perform` 函数拦截 `--dry-run` 模式，输出 `[DRY-RUN] Would update/delete/download` 提示并推进进度计数，不产生实际外部修改。
7. **`cmd/enhanced_features_test.go`**（新增）：
   - 编写针对 `--dry-run` 拦截机制、增量校验缓存命中与 Worker Pool 并发执行的覆盖性测试。
8. **`.gitignore`**：
   - 忽略 `theme`, `theme.exe`, `.themekit_cache.json`，确保工作区干净。

---

## 四、自测记录

### 1. 编译与格式测试
- **测试命令**：`go build -o theme github.com/Shopify/themekit/cmd/theme`
- **结果**：编译通过，生成轻量静态二进制可执行文件 `theme`。

### 2. 命令行启动与参数校验
- **测试命令**：`./theme version`
  - **输出**：`ThemeKit 1.3.3 linux/amd64`
- **测试命令**：`./theme --help` 与 `./theme deploy --help`
  - **证据**：新增的全局参数 `--concurrency`, `--continue-on-error`, `--dry-run`, `--no-cache` 均已正确注册并展现完整说明文档。

### 3. 单元测试与回归测试（全量通过）
- **测试命令**：`go test -v ./src/shopify/...`
  - **结果**：PASS（包含新实现的 `TestChecksumCache_Basic`, `TestChecksumCache_Disabled` 等全部通过）。
- **测试命令**：`go test -v ./src/cmdutil/...`
  - **结果**：PASS（包含增强的 `TestExpandEnvironments` 逗号分隔与去重测试全量通过）。
- **测试命令**：`go test -v ./cmd/...`
  - **结果**：PASS（包含 `TestPerform_DryRun`, `TestGenerateActions_WithChecksumCache`, `TestDeploy_ConcurrencyWorkerPool` 等全部通过）。

### 4. 并发竞争检测（Race Detector）
- **测试命令**：`go test -race ./src/... ./cmd/...`
  - **结果**：所有包均以 code 0 退出，无任何数据竞争（data race）异常。

### 3. 各包测试通过数（2026-11-17 主线复跑补充）

- **命令**：`go test -v ./...`（主线复跑，2026-11-17）
- **结果**：11 个含测试的包全部 ok，0 失败；各包顶层测试数：cmd 28、src/shopify 32、src/release 20、src/file 18、src/cmdutil 15、src/env 10、src/httpify 5、src/static 4、src/ratelimiter 3、src/atom 2、src/util 1，合计 138；cmd/static、cmd/theme、cmd/tkrelease、src/colors、src/static/cmd 无测试文件。
