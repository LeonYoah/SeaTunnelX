# SeaTunnel 集群命令参考文档

本文档记录 SeaTunnel 集群的启动、停止命令及相关配置信息，供 Agent 进程管理器实现时参考。

## 部署环境类型

SeaTunnel 集群支持三种部署环境：

| 环境类型 | 说明 | 管理方式 |
|---------|------|---------|
| bare_metal | 物理机/VM | 通过 Agent 管理 |
| docker | Docker 容器 | 通过 Docker API 管理 |
| kubernetes | Kubernetes 集群 | 通过 K8s API 管理 |

## 部署模式

### 混合部署模式 (Hybrid)

- Master 和 Worker 运行在同一个进程中
- 启动后监听端口：
  - `5801` - 集群通信端口
  - `8080` - REST API 端口（≥2.3.9 版本）

### 分离部署模式 (Separated)

- Master 和 Worker 各自独立进程
- Master 监听端口：
  - `5801` - 集群通信端口
  - `8080` - REST API 端口（≥2.3.9 版本）
- Worker 监听端口：
  - `5802` - Worker 通信端口

## 启动命令

### 混合部署模式

```bash
$SEATUNNEL_HOME/bin/seatunnel-cluster.sh -d
```

### 分离部署模式

**启动 Master：**
```bash
$SEATUNNEL_HOME/bin/seatunnel-cluster.sh -d -r master
```

**启动 Worker：**
```bash
$SEATUNNEL_HOME/bin/seatunnel-cluster.sh -d -r worker
```

### 命令参数说明

| 参数 | 说明 |
|-----|------|
| `-d` | 后台运行（daemon 模式） |
| `-r master` | 指定角色为 master |
| `-r worker` | 指定角色为 worker |
| `-cn <name>` | 指定集群名称 |

## 停止命令

### 停止逻辑

通过查找进程名 `org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer` 来定位进程：

- **混合部署**：直接匹配进程名（不带 `-r` 参数）
- **分离部署**：匹配进程名 + `-r master` 或 `-r worker`

### 停止脚本参考

```bash
#!/bin/bash

SEATUNNEL_DEFAULT_CLUSTER_NAME="seatunnel_default_cluster"
APP_MAIN="org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer"

# 停止混合部署集群
stop_hybrid() {
    local cluster_name="${1:-$SEATUNNEL_DEFAULT_CLUSTER_NAME}"
    if [ "$cluster_name" = "$SEATUNNEL_DEFAULT_CLUSTER_NAME" ]; then
        RES=$(ps -ef | grep "$APP_MAIN" | grep -v "\-cn\|\--cluster" | grep -v grep | awk '{print $2}')
    else
        RES=$(ps -ef | grep "$APP_MAIN" | grep "$cluster_name" | grep -v grep | awk '{print $2}')
    fi
    
    for pid in $RES; do
        kill $pid >/dev/null 2>&1
        echo "Killed process $pid"
    done
}

# 停止分离部署集群的指定角色
stop_separated() {
    local role="$1"  # master 或 worker
    local cluster_name="${2:-$SEATUNNEL_DEFAULT_CLUSTER_NAME}"
    
    if [ "$cluster_name" = "$SEATUNNEL_DEFAULT_CLUSTER_NAME" ]; then
        RES=$(ps -ef | grep "$APP_MAIN" | grep "\-r $role" | grep -v grep | awk '{print $2}')
    else
        RES=$(ps -ef | grep "$APP_MAIN" | grep "$cluster_name" | grep "\-r $role" | grep -v grep | awk '{print $2}')
    fi
    
    for pid in $RES; do
        kill $pid >/dev/null 2>&1
        echo "Killed $role process $pid"
    done
}
```

## 进程状态检查

### 检查进程是否运行

```bash
# 混合部署
ps -ef | grep "org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer" | grep -v "\-r" | grep -v grep

# 分离部署 - Master
ps -ef | grep "org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer" | grep "\-r master" | grep -v grep

# 分离部署 - Worker
ps -ef | grep "org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer" | grep "\-r worker" | grep -v grep
```

### 检查端口监听

```bash
# 检查 5801 端口（Master/Hybrid）
netstat -tlnp | grep 5801

# 检查 5802 端口（Worker）
netstat -tlnp | grep 5802

# 检查 8080 端口（REST API，≥2.3.9）
netstat -tlnp | grep 8080
```



### 查看日志

#### 分离模式
```bash
# master查看日志
tail -f $SEATUNNEL_HOME/logs/seatunnel-engine-master.log

# worker查看日志
tail -f $SEATUNNEL_HOME/logs/seatunnel-engine-worker.log


# 服务端查看日志
tail -f $SEATUNNEL_HOME/logs/seatunnel-engine-server.log
```

## Agent 进程管理器实现要点

在实现 `agent/internal/process/manager.go` 时需要考虑：

1. **启动流程**：
   - 根据部署模式和节点角色构建启动命令
   - 使用 `-d` 参数后台启动
   - 等待进程启动完成
   - 验证端口监听状态
   - 调用集群api查看集群状态和信息

2. **停止流程**：
   - 先发送 SIGTERM 信号
   - 等待进程优雅关闭（最长 30 秒）
   - 超时后发送 SIGKILL 强制终止

3. **状态检查**：
   - 通过进程名匹配检查进程是否存在
   - 通过端口检查验证服务是否正常

4. **配置参数**：
   - `SEATUNNEL_HOME` - SeaTunnel 安装目录
   - 集群名称（可选）
   - 部署模式（hybrid/separated）
   - 节点角色（master/worker，仅分离模式）

## 相关代码位置

- 集群模型定义：`internal/apps/cluster/model.go`
- 集群服务：`internal/apps/cluster/service.go`
- Agent 进程管理器（待实现）：`agent/internal/process/manager.go`
