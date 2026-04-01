# 部署说明

## 配置（Consul）

`docker-compose` 中的 **config-init** 会将 `deploy/config.docker.yaml` 推送到 Consul。启动前请在本机编辑该文件：

- `openai.api_key`、`openai.base_url`、`openai.model`：AI 与向量嵌入
- `email.*`：若使用「邮箱验证码注册」，需填写 SMTP

**不要**将含真实 API Key、SMTP 密码的 `config.docker.yaml` 提交到公开 Git 仓库；团队可各自维护本地副本或使用私有配置源。

## 常用命令

```bash
# 项目根目录
make docker-up      # 一键启动
make docker-down    # 停止并删除容器
make docker-logs    # 查看所有日志
make docker-ps      # 查看容器状态
```

## 排查容器异常

**看某个容器为什么退出：**

```bash
podman logs bookhive-mysql    # MySQL 启动失败看这里
podman logs bookhive-es       # Elasticsearch OOM 可看
```

**MySQL 一直 Exited (1)：** 多半是数据卷异常或 init 报错。清空数据卷再起：

```bash
make docker-down
podman volume rm deploy_mysql-data 2>/dev/null || true
make docker-up
```

**Exit 137（OOM）：** Podman / Docker 虚拟机内存不足。全栈（ES、Milvus、多应用容器）建议 **8GB 以上**，偏紧环境可调到 **16GB**：

```bash
podman machine stop
podman machine set --memory 16384
podman machine start
```

然后再 `make docker-up`。
