# openapi-cli

一个面向 Swagger 2.0 / OpenAPI 3.x 的命令行工具，用来把本地接口文档变成一个可搜索、可查看、可调用的 CLI。

它适合这类场景：

- 你手上有一份 `swagger.json` / `openapi.yaml`
- 你想在终端里快速搜索接口，不想每次打开 Swagger UI
- 你想直接根据文档调用接口
- 你想把多个项目的 API 文档统一管理在一个本地目录里

核心能力：

- 管理多个 API 项目
- 自动识别 Swagger 2.0 / OpenAPI 3.x
- 查看项目分类和接口列表
- 全局或项目内搜索接口
- 输出接口文档
- 直接调用接口
- 从本地文件或远程 URL 导入文档
- 从 `updateUrl` 同步最新 `api.json`
- 支持将请求打印成 `curl`
- 支持路径参数占位符：
  - `/users/{id}`
  - `/user-order/get/:orderId`

本文档覆盖目录结构、安装、全部命令、参数说明、示例和常见问题。

## 快速开始

### 1. 构建

```bash
go build -o openapi-cli .
```

### 2. 导入一个项目

本地文件：

```bash
./openapi-cli config create -p erp ./openapi.yaml \
  --base-url https://api.example.com
```

远程 URL：

```bash
./openapi-cli config create -p erp https://example.com/openapi.json \
  --base-url https://api.example.com \
  --auth-header Authorization \
  --auth-token 'Bearer xxx'
```

### 3. 搜索接口

```bash
./openapi-cli search -p erp -i user
```

### 4. 查看接口文档

```bash
./openapi-cli doc -p erp -i /biz/user-order/page
```

### 5. 调用接口

```bash
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```

## Release 下载

仓库提供 GitHub Releases 构建产物，发布后可直接下载常见平台二进制包。

计划产物：

- Linux `amd64`
- Linux `arm64`
- macOS `amd64`
- macOS `arm64`
- Windows `amd64`
- Windows `arm64`

打包格式：

- Linux / macOS: `tar.gz`
- Windows: `zip`

如果你要发布一个版本，只需要推送一个 tag：

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 会自动：

- 创建 Release
- 交叉编译常见平台二进制
- 将压缩包上传到该 Release

工作流文件见：

- [.github/workflows/release.yml](/Users/sofent/work/openapi-cli/.github/workflows/release.yml)

## 1. 安装与运行

在当前仓库中构建：

```bash
go build -o openapi-cli .
```

运行方式：

```bash
./openapi-cli help
```

说明：

- CLI 内部命令名为 `openapi`
- 如果你把二进制放进 `PATH` 并重命名为 `openapi`，也可以直接用 `openapi ...`
- 本文档示例统一使用当前仓库里的 `./openapi-cli`

## 2. 数据目录约定

默认读取用户目录下的：

```text
~/.openapi-cli/apis/
```

每个项目一个目录，例如：

```text
~/.openapi-cli/apis/erp/
├── api.json      # 或 api.yaml / api.yml
└── setting.json
```

说明：

- 项目目录名就是命令里的 `-p <project>`
- 第一版每个项目只维护一个接口文档文件
- 文档可以是 Swagger 2.0 或 OpenAPI 3.x
- 文档内容允许是 JSON 或 YAML
- `config create` 导入时统一写到项目目录下的 `api.json`

## 3. setting.json 结构

示例：

```json
{
  "baseUrl": "https://api.example.com",
  "authHeader": "Authorization",
  "authToken": "Bearer xxx",
  "updateUrl": "https://example.com/openapi.json",
  "specType": "openapi"
}
```

字段说明：

- `baseUrl`: 调用接口时的基础地址
- `authHeader`: 调用接口时自动注入的认证头名称
- `authToken`: 调用接口时自动注入的认证头值
- `updateUrl`: 文档同步地址，`config sync` 会从这里重新下载文档
- `specType`: 可选覆盖值。通常不需要填，CLI 会自动检测 Swagger / OpenAPI 类型

## 4. 功能总览

主命令：

- `help`
- `list`
- `search`
- `doc`
- `call`
- `config`

所有子命令都支持帮助：

```bash
./openapi-cli help
./openapi-cli list --help
./openapi-cli search --help
./openapi-cli doc --help
./openapi-cli call --help
./openapi-cli config --help
./openapi-cli config create --help
./openapi-cli config get --help
./openapi-cli config set --help
./openapi-cli config sync --help
```

## 5. 命令说明

### 5.1 `help`

查看根帮助，或查看某个子命令的帮助。

示例：

```bash
./openapi-cli help
./openapi-cli help config
./openapi-cli help call
```

### 5.2 `list`

列出所有项目，或者列出指定项目下的分类。

用法：

```bash
./openapi-cli list
./openapi-cli list -p erp
```

参数：

- `-p, --project`: 项目名。为空时列项目；有值时列项目分类

输出行为：

- 不传 `-p` 时，输出项目名列表
- 传 `-p` 时，输出 `分类名 (接口数)`
- 分类优先使用文档里的 `tags`
- 如果某个接口没有 `tags`，会归到 `default`

### 5.3 `search`

按名称、描述、路径、`operationId`、标签搜索接口。

用法：

```bash
./openapi-cli search -i login
./openapi-cli search -p erp -i lookup
./openapi-cli search -p erp -c users -i /users
```

参数：

- `-i, --interface`: 查询关键字，必填
- `-p, --project`: 项目名，可选；不传时全局搜索
- `-c, --category`: 分类/tag 过滤，可选

匹配规则：

- 大小写不敏感
- 搜索范围包括：
  - `summary`
  - `description`
  - `path`
  - `operationId`
  - `tags`

返回格式：

```text
project / tag1,tag2 / METHOD /path / summary
```

### 5.4 `doc`

输出接口文档。

用法：

```bash
./openapi-cli doc -p erp -i /users/{id}
./openapi-cli doc -p erp -i /users -m POST -t markdown
./openapi-cli doc -p erp -i /users -m POST -t openapi
./openapi-cli doc -p erp -i /users -m POST -t swagger
```

参数：

- `-p, --project`: 项目名，必填
- `-i, --interface`: 接口路径，必填，例如 `/users/{id}`
- `-m, --method`: HTTP 方法，可选
- `-t, --type`: 输出格式，可选，默认 `markdown`

`-m` 规则：

- 如果该路径在项目里只命中一个方法，`-m` 可以省略
- 如果同一路径下有多个方法，例如同时有 `GET` 和 `POST`，必须传 `-m`

`-t` 可选值：

- `markdown`: 人类可读文档
- `openapi`: 标准化后的 OpenAPI 风格输出
- `swagger`: Swagger 风格输出

说明：

- 如果原始文档是 Swagger 2.0，`-t swagger` 会尽量输出原生片段
- 如果原始文档是 OpenAPI 3.x，`-t swagger` 会回退到兼容的标准化输出

### 5.5 `call`

根据接口文档和项目配置发起真实 HTTP 请求。

用法：

```bash
./openapi-cli call -p erp -i /users/{id} -m GET -q '{"id":"1"}'
./openapi-cli call -p erp -i /users -m POST -d '{"name":"alice"}'
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}'
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```

参数：

- `-p, --project`: 项目名，必填
- `-i, --interface`: 接口路径，必填
- `-m, --method`: HTTP 方法，可选
- `-q, --query`: Query 参数 JSON 对象，可选
- `-d, --data`: 请求体 JSON 对象，可选
- `-v, --verbose`: 先把请求打印成 `curl` 再调用

请求构造规则：

- 请求地址：`baseUrl + path`
- `-q` 必须是 JSON 对象，例如 `{"pageNum":"1","pageSize":"20"}`
- `-d` 必须是 JSON 对象，作为 `application/json` 请求体发送
- 如果配置了 `authHeader` 和 `authToken`，会自动注入请求头

`-m` 规则：

- 路径唯一命中一个方法时，可省略
- 路径下有多个方法时，必须显式传 `-m`

`-v` 输出示例：

```text
Request:
curl -X "GET" -H "Authorization: Bearer xxx" "https://api.example.com/users?pageNum=1&pageSize=20"

Status: 200 OK
Headers:
...

Body:
...
```

响应输出：

- 状态码
- 响应头
- 响应体
- 如果响应体是 JSON，会自动格式化

### 5.6 `config`

管理项目配置，或导入/同步接口文档。

#### 5.6.1 查看项目配置

```bash
./openapi-cli config -p erp
```

返回完整 `setting.json` 内容。

#### 5.6.2 `config create`

创建项目并导入接口文档。

用法：

```bash
./openapi-cli config create -p erp ./swagger.json
./openapi-cli config create -p erp ./openapi.yaml
./openapi-cli config create -p erp https://example.com/swagger.json
./openapi-cli config create -p erp ./openapi.yaml \
  --base-url https://api.example.com \
  --auth-header Authorization \
  --auth-token 'Bearer xxx' \
  --update-url https://example.com/openapi.yaml \
  --spec-type openapi
```

参数：

- `-p, --project`: 项目名，必填
- `--base-url`: 初始化 `baseUrl`
- `--auth-header`: 初始化 `authHeader`
- `--auth-token`: 初始化 `authToken`
- `--update-url`: 初始化 `updateUrl`
- `--spec-type`: 初始化 `specType`

行为：

- 如果参数是本地文件路径，会复制文件内容到项目目录下的 `api.json`
- 如果参数是 URL，会先下载文件，再写入项目目录下的 `api.json`
- 如果来源是 URL，会自动把该 URL 写入 `updateUrl`
- 如果同时传了 `--update-url` 且来源也是 URL，最终以来源 URL 为准

#### 5.6.3 `config get`

读取单个配置项。

用法：

```bash
./openapi-cli config get -p erp baseUrl
./openapi-cli config get -p erp authHeader
./openapi-cli config get -p erp authToken
./openapi-cli config get -p erp updateUrl
```

支持的 key：

- `baseUrl`
- `authHeader`
- `authToken`
- `updateUrl`

#### 5.6.4 `config set`

更新单个配置项。

用法：

```bash
./openapi-cli config set -p erp baseUrl https://api.example.com
./openapi-cli config set -p erp authHeader Authorization
./openapi-cli config set -p erp authToken 'Bearer xxx'
./openapi-cli config set -p erp updateUrl https://example.com/swagger.json
```

支持的 key：

- `baseUrl`
- `authHeader`
- `authToken`
- `updateUrl`

#### 5.6.5 `config sync`

从 `updateUrl` 下载最新文档并覆盖本地 `api.json`。

用法：

```bash
./openapi-cli config sync -p erp
```

前提：

- `setting.json` 里已经配置了 `updateUrl`

行为：

- 从 `updateUrl` 下载最新接口文档
- 覆盖项目目录下的 `api.json`

## 6. 方法自动推断规则

`doc` 和 `call` 共享同一套方法选择规则：

- 如果路径只对应一个接口方法，自动使用这个方法
- 如果路径同时存在多个方法，命令会报错并提示你传 `-m`

示例：

```bash
./openapi-cli doc -p erp -i /biz/user-order/page
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}'
```

如果路径有多个方法：

```bash
./openapi-cli doc -p erp -i /users -m GET
./openapi-cli call -p erp -i /users -m POST -d '{"name":"alice"}'
```

## 7. 输出格式说明

### 7.1 `search` 输出

```text
erp / 用户管理 / GET /biz/user-info/page / 分页拉取用户信息
```

字段含义：

- 项目名
- 分类/tags
- HTTP 方法
- 路径
- 摘要

### 7.2 `doc -t markdown` 输出

包含：

- 方法
- 路径
- Summary
- Description
- Metadata
- Path / Query / Header 参数
- Request Body
- Responses

### 7.3 `call` 输出

包含：

- `Status`
- `Headers`
- `Body`

如果加了 `-v`，在最前面多一段：

- `Request`
- 一条对应请求的 `curl`

## 8. 实战示例

### 8.1 从远程地址创建项目

```bash
./openapi-cli config create -p erp https://example.com/openapi.json \
  --base-url https://api.example.com \
  --auth-header X-Api-Key \
  --auth-token 'xxxxxx'
```

### 8.2 查看项目分类

```bash
./openapi-cli list -p erp
```

### 8.3 搜索订单接口

```bash
./openapi-cli search -p erp -i order
```

### 8.4 查看接口文档

```bash
./openapi-cli doc -p erp -i /biz/user-order/page
```

### 8.5 调用订单分页接口

```bash
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}'
```

### 8.6 打印 curl 并调用

```bash
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```

### 8.7 同步最新接口文档

```bash
./openapi-cli config sync -p erp
```

## 9. 常见问题

### 9.1 `required flag(s) "project" not set`

缺少 `-p <项目名>`。

### 9.2 `interface not found`

说明你传的 `-i` 路径在该项目文档里没有找到精确匹配。

建议先搜索：

```bash
./openapi-cli search -p erp -i keyword
```

### 9.3 `multiple methods found for ...`

说明同一路径下存在多个方法，必须补 `-m`。

### 9.4 `project baseUrl is empty`

说明项目配置里还没有 `baseUrl`，可以这样设置：

```bash
./openapi-cli config set -p erp baseUrl https://api.example.com
```

### 9.5 接口返回 `Access token is empty`

说明调用已经发出，但项目配置里的认证信息不完整。

例如：

```bash
./openapi-cli config set -p erp authHeader Authorization
./openapi-cli config set -p erp authToken 'Bearer xxx'
```

或者：

```bash
./openapi-cli config set -p erp authHeader X-Api-Key
./openapi-cli config set -p erp authToken 'xxxxxx'
```

### 9.6 文档里有不规范字段，比如 `type: "null"`，会不会报错？

当前 CLI 采用“尽力解析”的策略：

- 只要文档能被解析并建立接口索引，就允许继续 `list/search/doc/call`
- 不会因为某些宽松 Swagger 字段直接把整个项目加载失败

## 10. 推荐工作流

### 方案一：从本地文档开始

```bash
./openapi-cli config create -p erp ./openapi.yaml --base-url https://api.example.com
./openapi-cli search -p erp -i order
./openapi-cli doc -p erp -i /biz/user-order/page
./openapi-cli call -p erp -i /biz/user-order/page -q '{"pageNum":"1","pageSize":"20"}' -v
```

### 方案二：从远程文档开始

```bash
./openapi-cli config create -p erp https://example.com/openapi.json \
  --base-url https://api.example.com \
  --auth-header Authorization \
  --auth-token 'Bearer xxx'

./openapi-cli config sync -p erp
./openapi-cli search -p erp -i user
```

## 11. 查看帮助

任何时候都可以用帮助命令查看实时参数：

```bash
./openapi-cli help
./openapi-cli help config
./openapi-cli help call
./openapi-cli config create --help
./openapi-cli call --help
```
