# 参与贡献

感谢你有意改进 **docker-proxy**。

## 报告问题

- 请尽量说明：Go / Docker 版本、操作系统、`config.yaml` 中与问题相关的路由（可打码敏感信息）、复现步骤与完整错误信息。

## 提交代码

1. Fork [github.com/Rehtt/docker-proxy](https://github.com/Rehtt/docker-proxy) 并创建分支：`git checkout -b feat/your-topic` 或 `fix/issue-description`。
2. 保持改动与 PR 描述 **聚焦单一主题**，避免无关格式化或大范围重排。
3. 提交前在本地执行：

   ```bash
   make vet
   make test
   make build
   ```

4. 提交信息使用清晰的中文或英文完整句，说明「做了什么、为什么」。

## 代码风格

- 与现有代码保持一致：命名、错误处理、包结构。
- 新增逻辑尽量附带可维护的单元测试（若适用）。
- 不引入未讨论过的大型依赖，除非有充分理由。

## 文档

- 行为或配置变更请同步更新 **README.md**（及示例 `config.example.yaml` 如有需要）。

再次感谢你的贡献。
