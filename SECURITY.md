# 安全策略

## 支持的版本

请以 [github.com/Rehtt/docker-proxy](https://github.com/Rehtt/docker-proxy) 默认分支上的最新提交为准；安全修复会在可行范围内同步说明。

## 报告漏洞

若你发现 **docker-proxy** 中存在安全问题，请 **不要** 在公开 Issue 中讨论利用细节。

请使用 GitHub 仓库的 **Security → Report a vulnerability**（若已开启 Private vulnerability reporting），或通过其它私密渠道联系维护者。

## 范围说明

- 上游 registry 或 Docker 引擎自身的漏洞，请向对应项目报告。
- 使用本代理时，请妥善保护 TLS 私钥与仅内网可用的 `insecure-registries` 配置。
