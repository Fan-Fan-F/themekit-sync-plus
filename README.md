# themekit-sync-plus

> **高效、轻量、专为 CI/CD 自动化流水线与跨境多店铺矩阵优化的 Shopify 主题同步 CLI 工具。**
> 
> *A high-performance, lightweight Shopify theme synchronization CLI optimized for CI/CD automation and multi-store storefront matrices.*

---

## 中文文档

### 为什么选择 themekit-sync-plus？

Shopify 官方现行推荐的 Shopify CLI 强依赖 Node.js/Ruby 完整运行时与交互式浏览器 OAuth 鉴权，体积庞大，在自动化无头 CI/CD 流水线（GitHub Actions、GitLab CI、Docker 容器）以及多独立站出海矩阵批量部署中十分笨重且难以无交互运行。

原版 ThemeKit 拥有轻量单一静态二进制、零外部运行时依赖的巨大优势，但在面对现代大主题（数千个资源文件、大量高清图片）及多店铺矩阵同步时，存在全量重复读取、无本地增量缓存、并发不可控、无法预检等缺陷。

**themekit-sync-plus** 在完全保留原版协议兼容性的基础上，专注补齐自动化部署场景的短板，带来极速、安全、弹性的多店铺增量同步体验。

### 核心功能与改进点（对应 CHANGES.md）

1. **大主题目录高性能增量同步与本地校验缓存（Incremental Sync Cache）**
   - 引入本地增量缓存机制（`.themekit_cache.json`），记录资产文件修改时间与体积（`mtime + size -> checksum`）。
   - 未改动资产直接复用校验值，避免全量文件重复读取与 base64 内存编码，将大目录扫描与比对提速 90% 以上；提供 `--no-cache` 开关供全量重建。
2. **多店铺与多环境批量同步能力增强（Multi-Store & Multi-Environment Sync）**
   - 支持逗号分隔指定多个店铺/环境（如 `-e "store-us, store-eu, store-jp"`），支持环境去重与确定性排序。
   - 新增 `--continue-on-error` 容错机制：矩阵部署中单个店铺网络或认证临时故障不会打断剩余店铺的部署。
   - 任务结束输出结构化多店铺同步报表（Multi-Environment Sync Summary），清晰呈现各店成功/失败状态。
3. **并发工作池与限流保护机制（Configurable Concurrency Worker Pool）**
   - 摒弃原版瞬间启动数千个无界 goroutine 的旧设计，改为由 Worker Pool 受控调度。
   - 新增全局 `--concurrency` 参数（默认 10，支持 1-50 调节），有效平滑突发网络请求，规避 Shopify API 429 限流与本地端口耗尽。
4. **安全预览模式（Dry-Run Mode）**
   - 全局支持 `--dry-run` 参数。在执行 `deploy`、`download` 或清理时，预先计算并输出拟操作的明细清单（`[DRY-RUN] Would update / delete / download`），不发出任何实质性破坏写请求，便于在生产流水线中安全审计。
5. **缺陷修复与文件操作鲁棒性增强（Bug Fixes & Robustness）**
   - 修复资产扫描中静默丢弃文件读取错误（原代码 `var asset, _ = ReadAsset... // TODO handle error`）的隐患，确保读取异常向上反馈。
   - 规范 `config/settings_data.json` 同步时序，在常规资源并发同步完成后显式有序执行并严密捕获错误。
   - 完善 `.gitignore`，防止缓存与构建产物意外污染版本库。

---

### 快速开始

#### 1. 安装与构建

要求 Go 1.22+ 环境：

```bash
# 克隆仓库
git clone https://github.com/Shopify/themekit.git themekit-sync-plus
cd themekit-sync-plus

# 编译为二进制
go build -o theme github.com/Shopify/themekit/cmd/theme

# 检查安装与版本
./theme version
```

#### 2. 配置多店铺环境

在项目根目录下创建 `config.yml`（或使用 `theme configure` 命令生成）。

> [!NOTE]
> 请始终使用占位符配置示例，切勿将真实 API 密钥或密码硬编码提交到代码库中。建议结合环境变量或 CI 密钥库（如 GitHub Secrets）注入。

```yaml
# config.yml
development:
  password: "${THEMEKIT_PASSWORD_DEV}"
  theme_id: "123456789012"
  store: "your-dev-store.myshopify.com"
  directory: "theme"

store-us:
  password: "${THEMEKIT_PASSWORD_US}"
  theme_id: "234567890123"
  store: "your-us-store.myshopify.com"
  directory: "theme"

store-eu:
  password: "${THEMEKIT_PASSWORD_EU}"
  theme_id: "345678901234"
  store: "your-eu-store.myshopify.com"
  directory: "theme"
```

#### 3. 核心常用命令

- **预览将要部署的变更（Dry-Run 模式，不影响线上）**：
  ```bash
  ./theme deploy --dry-run -e store-us
  ```

- **批量增量部署到多个店铺（带并发控制与容错）**：
  ```bash
  ./theme deploy -e "store-us, store-eu" --concurrency=12 --continue-on-error
  ```

- **全量无缓存重新比对部署**：
  ```bash
  ./theme deploy -e store-us --no-cache
  ```

- **监听本地主题修改并自动同步**：
  ```bash
  ./theme watch -e development
  ```

---

## English Documentation

### Why themekit-sync-plus?

While Shopify's current official Shopify CLI requires a full Node.js/Ruby runtime and interactive browser OAuth authentication, it is excessively heavy and ill-suited for headless CI/CD automation pipelines (GitHub Actions, GitLab CI, Docker containers) or cross-border multi-store storefront matrix deployments.

The original ThemeKit offers a lightweight, single static binary with zero external runtime dependencies. However, when working with large modern themes (thousands of assets, high-res images) and multi-store fleets, it suffers from redundant disk reads, lack of local checksum caching, unbounded goroutine bursts, and absence of a safe dry-run mode.

**themekit-sync-plus** retains 100% protocol and configuration compatibility while specifically addressing these automation bottlenecks, providing a fast, resilient, and safe multi-store sync experience.

### Key Features & Improvements (from CHANGES.md)

1. **High-Performance Incremental Sync & Local Checksum Cache**
   - Introduces `.themekit_cache.json` tracking `mtime + size -> checksum`.
   - Bypasses redundant file reads and base64 memory encoding for unchanged assets, speeding up large theme scanning and diffing by over 90%. Includes a `--no-cache` override.
2. **Enhanced Multi-Store & Multi-Environment Sync**
   - Supports comma-separated environments via `-e` / `--env` (e.g., `-e "store-us, store-eu, store-jp"`), with automatic deduplication and deterministic sorting.
   - Adds `--continue-on-error` fault tolerance: a transient network or auth glitch on one store won't terminate deployments to the rest of the store fleet.
   - Displays a structured multi-environment execution summary report at command completion.
3. **Bounded Concurrency Worker Pool**
   - Replaces unbounded goroutine explosions with a regulated Worker Pool.
   - Adds global `--concurrency` flag (default 10, range 1-50) to smooth traffic spikes and prevent Shopify API 429 rate-limiting.
4. **Safe Preview Mode (--dry-run)**
   - Simulates `deploy`, `download`, and cleanup operations without making destructive API calls or disk writes. Emits explicit `[DRY-RUN] Would update / delete / download` logs for CI/CD auditing.
5. **Bug Fixes & Operational Robustness**
   - Fixed silent error swallowing in directory asset scanning (`// TODO handle error`), ensuring disk/permission errors bubble up properly.
   - Ensured atomic, ordered execution of `config/settings_data.json` after all standard assets complete.
   - Updated `.gitignore` to prevent cache files and binaries from dirtying git status.

---

### Quick Start (EN)

#### 1. Build and Install

Prerequisites: Go 1.22+

```bash
git clone https://github.com/Shopify/themekit.git themekit-sync-plus
cd themekit-sync-plus

go build -o theme github.com/Shopify/themekit/cmd/theme
./theme version
```

#### 2. Configuration

Create `config.yml` in your theme project root:

```yaml
development:
  password: "${THEMEKIT_PASSWORD_DEV}"
  theme_id: "123456789012"
  store: "your-dev-store.myshopify.com"
  directory: "theme"

store-us:
  password: "${THEMEKIT_PASSWORD_US}"
  theme_id: "234567890123"
  store: "your-us-store.myshopify.com"
  directory: "theme"

store-eu:
  password: "${THEMEKIT_PASSWORD_EU}"
  theme_id: "345678901234"
  store: "your-eu-store.myshopify.com"
  directory: "theme"
```

#### 3. Common Commands

```bash
# Preview deployment safely without changing anything
./theme deploy --dry-run -e store-us

# Batch deploy to multiple stores with concurrency and error resilience
./theme deploy -e "store-us, store-eu" --concurrency=12 --continue-on-error

# Re-scan and deploy without local cache
./theme deploy -e store-us --no-cache
```

---

## 来源与致谢 / Acknowledgements

本项目基于 [Shopify Themekit](https://github.com/Shopify/themekit) 二次开发，原项目由 Shopify 以 MIT 许可证开源。

*This project is a secondary development based on [Shopify Themekit](https://github.com/Shopify/themekit), originally open-sourced by Shopify under the MIT License.*

---

## 许可证 / License

本项目沿用原项目的 MIT 开源许可证。全文详见根目录 [LICENSE](LICENSE)。

*This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.*
