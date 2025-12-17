#!/bin/bash

# 确保遇到错误时立即退出
set -e

# 预检测 --json 参数（在加载其他内容前）
_JSON_MODE=false
for arg in "$@"; do
    [ "$arg" = "--json" ] && _JSON_MODE=true && break
done

# 获取脚本执行路径
EXEC_PATH=$(cd "$(dirname "$0")" && pwd)
[ "$_JSON_MODE" != "true" ] && echo "执行路径: $EXEC_PATH"

# 记录开始时间
START_TIME=$(date +%s)

# 日志文件路径
LOG_DIR="$EXEC_PATH/seatunnel-install-log-${INSTALL_USER:-$(whoami)}"
LOG_FILE="$LOG_DIR/install.log"

# yq相关控制
USE_YQ=false
COMMAND_LIB_DIR="$EXEC_PATH/lib"

# 最大重试次数
MAX_RETRIES=3
# SSH超时时间(秒)
SSH_TIMEOUT=10

# 加载通用工具函数库
if [ -f "$EXEC_PATH/util.sh" ]; then
    source "$EXEC_PATH/util.sh"
else
    echo "[ERROR] 找不到 util.sh 文件: $EXEC_PATH/util.sh"
    exit 1
fi

# 安装包仓库地址映射
declare -A PACKAGE_REPOS=(
    ["apache"]="https://archive.apache.org/dist/seatunnel"
    ["aliyun"]="https://mirrors.aliyun.com/apache/seatunnel"
    ["huaweicloud"]="https://mirrors.huaweicloud.com/apache/seatunnel"
)

# 插件仓库地址映射
declare -A PLUGIN_REPOS=(
    ["apache"]="https://repo1.maven.org/maven2"
    ["aliyun"]="https://maven.aliyun.com/repository/public"
    ["huaweicloud"]="https://repo.huaweicloud.com/repository/maven"
)


# 添加错误处理函数
handle_error() {
    local exit_code=$?
    local line_number=$1
    # 避免递归错误
    if [ "${IN_ERROR_HANDLER:-0}" -eq 1 ]; then
        echo "致命错误: 在错误处理过程中发生错误"
        exit 1
    fi
    export IN_ERROR_HANDLER=1
    
    log_error "脚本在第 $line_number 行发生错误 (退出码: $exit_code)"
    
    # 清理并退出
    cleanup
    exit $exit_code
}

# 设置错误处理
trap 'handle_error ${LINENO}' ERR

# 增强的清理函数
cleanup() {
    local exit_code=$?
    # 避免重复清理
    if [ "${IN_CLEANUP:-0}" -eq 1 ]; then
        return
    fi
    export IN_CLEANUP=1
    
    log_info "开始清理..."
    
    # 清理临时文件
    cleanup_temp_files
    
    # 如果安装失败,提示用户
    if [ $exit_code -ne 0 ]; then
        log_warning "安装失败。如果需要重新安装,请手动删除安装目录: $SEATUNNEL_HOME"
        log_warning "删除命令: sudo rm -rf $SEATUNNEL_HOME"
    fi
    
    exit $exit_code
}

# 设置清理trap
trap cleanup EXIT INT TERM

# 读取配置文件
read_config() {
    local config_file="$EXEC_PATH/config.properties"
    check_file "$config_file"
    
    # 读取基础配置
    SEATUNNEL_VERSION=$(grep "^SEATUNNEL_VERSION=" "$config_file" | cut -d'=' -f2)
    BASE_DIR=$(grep "^BASE_DIR=" "$config_file" | cut -d'=' -f2)
    SSH_PORT=$(grep "^SSH_PORT=" "$config_file" | cut -d'=' -f2)
    DEPLOY_MODE=$(grep "^DEPLOY_MODE=" "$config_file" | cut -d'=' -f2)
    # 读取用户配置
    INSTALL_USER=$(grep "^INSTALL_USER=" "$config_file" | cut -d'=' -f2)
    INSTALL_GROUP=$(grep "^INSTALL_GROUP=" "$config_file" | cut -d'=' -f2)
    
    # 设置下载目录
    DOWNLOAD_DIR="${BASE_DIR}/downloads"
    mkdir -p "$DOWNLOAD_DIR"
    setup_permissions "$DOWNLOAD_DIR"
    

    
    # 根据部署模式读取节点配置
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式：读取所有集群节点
        CLUSTER_NODES_STRING=$(grep "^CLUSTER_NODES=" "$config_file" | cut -d'=' -f2)
        [[ -z "$CLUSTER_NODES_STRING" ]] && log_error "CLUSTER_NODES 未配置"
        IFS=',' read -r -a ALL_NODES <<< "$CLUSTER_NODES_STRING"
    else
        # 分离模式：读取master和worker节点
        MASTER_IPS_STRING=$(grep "^MASTER_IP=" "$config_file" | cut -d'=' -f2)
        WORKER_IPS_STRING=$(grep "^WORKER_IPS=" "$config_file" | cut -d'=' -f2)
        [[ -z "$MASTER_IPS_STRING" ]] && log_error "MASTER_IP 未配置"
        [[ -z "$WORKER_IPS_STRING" ]] && log_error "WORKER_IPS 未配置"
        
        # 转换为数组
        IFS=',' read -r -a MASTER_IPS <<< "$MASTER_IPS_STRING"
        IFS=',' read -r -a WORKER_IPS <<< "$WORKER_IPS_STRING"
        ALL_NODES=("${MASTER_IPS[@]}" "${WORKER_IPS[@]}")
    fi
    
    # 设置SEATUNNEL_HOME
    SEATUNNEL_HOME="$BASE_DIR/apache-seatunnel-$SEATUNNEL_VERSION"
    
    # 验证必要的配置
    [[ -z "$SEATUNNEL_VERSION" ]] && log_error "SEATUNNEL_VERSION 未配置"
    [[ -z "$BASE_DIR" ]] && log_error "BASE_DIR 未配置"
    
    # 添加用户配置验证
    [[ -z "$INSTALL_USER" ]] && log_error "INSTALL_USER 未配置"
    [[ -z "$INSTALL_GROUP" ]] && log_error "INSTALL_GROUP 未配置"
    
    # 验证部署模式
    if [[ "$DEPLOY_MODE" != "hybrid" && "$DEPLOY_MODE" != "separated" ]]; then
        log_error "DEPLOY_MODE 必须是 hybrid:混合模式 或 separated:分离模式"
    fi
    
    # 读取JVM内存配置
    HYBRID_HEAP_SIZE=$(grep "^HYBRID_HEAP_SIZE=" "$config_file" | cut -d'=' -f2)
    MASTER_HEAP_SIZE=$(grep "^MASTER_HEAP_SIZE=" "$config_file" | cut -d'=' -f2)
    WORKER_HEAP_SIZE=$(grep "^WORKER_HEAP_SIZE=" "$config_file" | cut -d'=' -f2)
    
    # 验证内存配置
    [[ -z "$HYBRID_HEAP_SIZE" ]] && log_error "HYBRID_HEAP_SIZE 未配置"
    [[ -z "$MASTER_HEAP_SIZE" ]] && log_error "MASTER_HEAP_SIZE 未配置"
    [[ -z "$WORKER_HEAP_SIZE" ]] && log_error "WORKER_HEAP_SIZE 未配置"
    
    # 读取安装模式配置
    INSTALL_MODE=$(grep "^INSTALL_MODE=" "$config_file" | cut -d'=' -f2)
    [[ -z "$INSTALL_MODE" ]] && log_error "INSTALL_MODE 未配置"
    
    # 验证安装模式
    if [[ "$INSTALL_MODE" != "online" && "$INSTALL_MODE" != "offline" ]]; then
        log_error "INSTALL_MODE 必须是 online:在线安装 或 offline:离线安装"
    fi
    
    # 读取安装包相关配置
    if [[ "$INSTALL_MODE" == "offline" ]]; then
        PACKAGE_PATH=$(grep "^PACKAGE_PATH=" "$config_file" | cut -d'=' -f2)
        [[ -z "$PACKAGE_PATH" ]] && log_error "离线安装模式下 PACKAGE_PATH 未配置"
        
        # 处理版本号变量
        PACKAGE_PATH=$(echo "$PACKAGE_PATH" | sed "s/\${SEATUNNEL_VERSION}/$SEATUNNEL_VERSION/g")
        
        # 转换为绝对路径
        if [[ "$PACKAGE_PATH" != /* ]]; then
            PACKAGE_PATH="$EXEC_PATH/$PACKAGE_PATH"
        fi
    else
        # 读取安装包仓库配置
        PACKAGE_REPO=$(grep "^PACKAGE_REPO=" "$config_file" | cut -d'=' -f2)
        PACKAGE_REPO=${PACKAGE_REPO:-aliyun}  # 默认使用aliyun源
        
        # 验证仓库配置
        if [[ "$PACKAGE_REPO" == "custom" ]]; then
            CUSTOM_PACKAGE_URL=$(grep "^CUSTOM_PACKAGE_URL=" "$config_file" | cut -d'=' -f2)
            [[ -z "$CUSTOM_PACKAGE_URL" ]] && log_error "使用自定义仓库(PACKAGE_REPO=custom)时必须配置 CUSTOM_PACKAGE_URL"
        else
            [[ -z "${PACKAGE_REPOS[$PACKAGE_REPO]}" ]] && log_error "不支持的安装包仓库: $PACKAGE_REPO"
        fi
    fi

    # 读取连接器配置
    INSTALL_CONNECTORS=$(grep "^INSTALL_CONNECTORS=" "$config_file" | cut -d'=' -f2)
    INSTALL_CONNECTORS=${INSTALL_CONNECTORS:-true}  # 默认安装

    if [ "$INSTALL_CONNECTORS" = "true" ]; then
        CONNECTORS=$(grep "^CONNECTORS=" "$config_file" | cut -d'=' -f2)
        PLUGIN_REPO=$(grep "^PLUGIN_REPO=" "$config_file" | cut -d'=' -f2)
        PLUGIN_REPO=${PLUGIN_REPO:-aliyun}  # 默认使用aliyun
    fi
    
    # 读取检查点存储配置
    CHECKPOINT_STORAGE_TYPE=$(grep "^CHECKPOINT_STORAGE_TYPE=" "$config_file" | cut -d'=' -f2)
    CHECKPOINT_NAMESPACE=$(grep "^CHECKPOINT_NAMESPACE=" "$config_file" | cut -d'=' -f2)
    
    # 根据存储类型读取相应配置
    case "$CHECKPOINT_STORAGE_TYPE" in
        "HDFS")
            HDFS_NAMENODE_HOST=$(grep "^HDFS_NAMENODE_HOST=" "$config_file" | cut -d'=' -f2)
            HDFS_NAMENODE_PORT=$(grep "^HDFS_NAMENODE_PORT=" "$config_file" | cut -d'=' -f2)
            [[ -z "$HDFS_NAMENODE_HOST" ]] && log_error "HDFS模式下必须配置 HDFS_NAMENODE_HOST"
            [[ -z "$HDFS_NAMENODE_PORT" ]] && log_error "HDFS模式下必须配置 HDFS_NAMENODE_PORT"
            ;;
        "OSS"|"S3")
            STORAGE_ENDPOINT=$(grep "^STORAGE_ENDPOINT=" "$config_file" | cut -d'=' -f2)
            STORAGE_ACCESS_KEY=$(grep "^STORAGE_ACCESS_KEY=" "$config_file" | cut -d'=' -f2)
            STORAGE_SECRET_KEY=$(grep "^STORAGE_SECRET_KEY=" "$config_file" | cut -d'=' -f2)
            STORAGE_BUCKET=$(grep "^STORAGE_BUCKET=" "$config_file" | cut -d'=' -f2)
            [[ -z "$STORAGE_ENDPOINT" ]] && log_error "${CHECKPOINT_STORAGE_TYPE}模式下必须配置 STORAGE_ENDPOINT"
            [[ -z "$STORAGE_ACCESS_KEY" ]] && log_error "${CHECKPOINT_STORAGE_TYPE}模式下必须配置 STORAGE_ACCESS_KEY"
            [[ -z "$STORAGE_SECRET_KEY" ]] && log_error "${CHECKPOINT_STORAGE_TYPE}模式下必须配置 STORAGE_SECRET_KEY"
            [[ -z "$STORAGE_BUCKET" ]] && log_error "${CHECKPOINT_STORAGE_TYPE}模式下必须配置 STORAGE_BUCKET"
            ;;
        "LOCAL_FILE")
            # 本地文件模式下使用默认路径
            CHECKPOINT_NAMESPACE="$SEATUNNEL_HOME/checkpoint"
            ;;
        "")
            log_error "必须配置 CHECKPOINT_STORAGE_TYPE"
            ;;
        *)
            log_error "不支持的检查点存储类型: $CHECKPOINT_STORAGE_TYPE"
            ;;
    esac

    # 读取开机自启动配置
    ENABLE_AUTO_START=$(grep "^ENABLE_AUTO_START=" "$config_file" | cut -d'=' -f2)
    ENABLE_AUTO_START=${ENABLE_AUTO_START:-true}  

    # 读取连接器配置
    INSTALL_CONNECTORS=$(grep "^INSTALL_CONNECTORS=" "$config_file" | cut -d'=' -f2)
    INSTALL_CONNECTORS=${INSTALL_CONNECTORS:-true}  # 默认安装
    
    if [ "$INSTALL_CONNECTORS" = "true" ]; then
        CONNECTORS=$(grep "^CONNECTORS=" "$config_file" | cut -d'=' -f2)
        PLUGIN_REPO=$(grep "^PLUGIN_REPO=" "$config_file" | cut -d'=' -f2)
        PLUGIN_REPO=${PLUGIN_REPO:-aliyun}  # 默认使用aliyun
    fi
    
    # 读取端口配置
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        HYBRID_PORT=$(grep "^HYBRID_PORT=" "$config_file" | cut -d'=' -f2)
        HYBRID_PORT=${HYBRID_PORT:-5801}  # 默认端口5801
    else
        MASTER_PORT=$(grep "^MASTER_PORT=" "$config_file" | cut -d'=' -f2)
        WORKER_PORT=$(grep "^WORKER_PORT=" "$config_file" | cut -d'=' -f2)
        MASTER_PORT=${MASTER_PORT:-5801}  # 默认端口5801
        WORKER_PORT=${WORKER_PORT:-5802}  # 默认端口5802
    fi
    
    # 读取HTTP端口(2.3.9+)
    MASTER_HTTP_PORT=$(grep "^MASTER_HTTP_PORT=" "$config_file" | cut -d'=' -f2)
    MASTER_HTTP_PORT=${MASTER_HTTP_PORT:-8080}
    
    # 读取安全检查配置
    CHECK_FIREWALL=$(grep "^CHECK_FIREWALL=" "$config_file" | cut -d'=' -f2)
    CHECK_FIREWALL=${CHECK_FIREWALL:-true}  # 默认为true
    FIREWALL_CHECK_ACTION=$(grep "^FIREWALL_CHECK_ACTION=" "$config_file" | cut -d'=' -f2)
    FIREWALL_CHECK_ACTION=${FIREWALL_CHECK_ACTION:-error}  # 默认为error
}

# 检查用户配置
check_user() {
    
    # 检查当前执行用户和配置的安装用户是否一致
    local current_user=$(whoami)
    if [ "$current_user" != "$INSTALL_USER" ]; then
        log_error "当前执行用户($current_user)与配置文件中的安装用户($INSTALL_USER)不一致。

请执行以下操作之一:

1. 如果用户不存在，请按照以下命令创建用户:
   sudo groupadd $INSTALL_GROUP
   sudo useradd -m -g $INSTALL_GROUP $INSTALL_USER
   sudo passwd $INSTALL_USER
   
   ## 配置sudo权限（推荐使用以下方式）：
   # 创建sudo权限配置文件（免密配置）：
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   Defaults:$INSTALL_USER !authenticate
   $INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL
   EOF
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER
   
   # 验证sudo免密是否生效：
   su - $INSTALL_USER
   sudo whoami   # 应该显示root且不提示密码
   sudo ls /root # 应该能访问root目录且不提示密码
   sudo systemctl status # 应该能执行系统管理命令且不提示密码

   ## 多节点部署需要配置SSH免密登录：
   su - $INSTALL_USER  # 切换到安装用户
   ssh-keygen -t rsa   # 生成密钥对，一路回车即可
   
   # 对所有节点执行以下命令（包括本机）：
   ssh-copy-id $INSTALL_USER@node1
   ssh-copy-id $INSTALL_USER@node2
   # ... 对所有节点执行
    
   # 验证免密登录：
   ssh node1 "whoami"  # 应该显示用户名且无需密码
   ssh node2 "whoami"
   
   ## 注意：
   - node1、node2替换为实际的节点主机名或IP
   - 首次SSH连接会提示确认指纹，输入yes即可
   - 需要输入目标节点上$INSTALL_USER的密码
   - 所有节点上都需要先创建好相同的用户和sudo权限

2. 切换到配置的安装用户:
   su - $INSTALL_USER

3. 或修改配置文件中的安装用户:
   vim config.properties
   # 修改 INSTALL_USER=$current_user

注意: 
- sudo权限配置说明:
-  * -a: 追加模式，不删除已有组
-  * -G: 指定附加组
-  * sudo/wheel: 系统管理员组（不同系统名称可能不同）
- 建议使用sudoers.d配置sudo权限，这样:
  * 不依赖系统sudo组
  * 无需输入密码
  * 权限更精确可控
  * 便于管理和移除
- 如果使用root用户，请将配置文件中的INSTALL_USER也设置为root
- 多节点部署必须配置SSH免密登录
- 所有节点的用户名、用户组、sudo权限配置必须一致"
        exit 1
    fi
    
    # 检查本地和远程节点的sudo权限
    check_sudo_permission() {
        local node=$1
        local is_remote=$2
        
        # 定义sudo权限验证命令
        local verify_commands=(
            "sudo whoami | grep -w root"         # 确认可以获取root权限
            "sudo ls /root &>/dev/null"          # 测试访问root目录
            "sudo systemctl status &>/dev/null"  # 测试systemctl权限
        )
        
        # 检查sudo是否需要密码
        check_sudo_nopasswd() {
            local node=$1
            local is_remote=$2
            
            if [ "$is_remote" = true ]; then
                # 远程节点检查
                if ! ssh_with_retry "$node" "sudo -n true" 2>/dev/null; then
                    return 1
                fi
            else
                # 本地节点检查
                if ! sudo -n true 2>/dev/null; then
                    return 1
                fi
            fi
            return 0
        }
        
        if [ "$is_remote" = true ]; then
            # 远程节点检查
            local sudo_ok=true
            # 先检查sudo免密
            if ! check_sudo_nopasswd "$node" true; then
                log_error "远程节点 $node 的用户($INSTALL_USER)的sudo权限需要输入密码，请按以下步骤配置sudo免密：

1. 在节点 $node 上执行以下命令：
   
   # 创建或编辑sudo配置文件
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   $INSTALL_USER ALL=(ALL) NOPASSWD: ALL
   EOF
   
   # 设置正确的权限
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER

2. 验证sudo免密：
   sudo -n true  # 应该不提示输入密码
   
注意：
- 必须配置sudo免密，否则自动化部署可能失败
- 如果/etc/sudoers.d/$INSTALL_USER已存在，请确保包含NOPASSWD设置
- 某些系统可能需要在/etc/sudoers中启用includedir /etc/sudoers.d"
                return 1
            fi
            
            for cmd in "${verify_commands[@]}"; do
                if ! ssh_with_retry "$node" "$cmd"; then
                    sudo_ok=false
                    break
                fi
            done
            
            if [ "$sudo_ok" = false ]; then
                log_error "远程节点 $node 的用户($INSTALL_USER)没有sudo权限，请执行以下步骤：

1. 在节点 $node 上执行以下命令之一：

   方式1：将用户添加到sudo组（部分系统可能是wheel组）
   sudo usermod -aG sudo $INSTALL_USER  # Ubuntu/Debian系统
   # 或
   sudo usermod -aG wheel $INSTALL_USER # CentOS/RHEL系统

   方式2：创建sudo权限配置文件（推荐）
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   Defaults:$INSTALL_USER !authenticate
   $INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL
   EOF
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER

2. 验证sudo权限（以下命令都应该成功执行）：
   sudo whoami             # 应该输出 root
   sudo ls /root           # 应该能访问root目录
   sudo systemctl status   # 应该能执行系统管理命令

注意：
- 建议使用方式2配置sudo权限，这样无需输入密码
- 如果使用方式1，某些系统可能使用wheel组
- 添加权限后需要重新登录生效
- 确保授予的权限足够安装和管理服务"
                return 1
            fi
        else
            # 本地节点检查
            local sudo_ok=true
            # 先检查sudo免密
            if ! check_sudo_nopasswd "$node" false; then
                log_error "当前用户($INSTALL_USER)的sudo权限需要输入密码，请按以下步骤配置sudo免密：

1. 创建或编辑sudo配置文件：
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   Defaults:$INSTALL_USER !authenticate
   $INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL
   EOF
   
2. 设置正确的权限：
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER

3. 验证sudo免密：
   sudo -n true  # 应该不提示输入密码
   
注意：
- 必须配置sudo免密，否则自动化部署可能失败
- 如果/etc/sudoers.d/$INSTALL_USER已存在，请确保包含NOPASSWD设置
- 某些系统可能需要在/etc/sudoers中启用includedir /etc/sudoers.d"
                return 1
            fi
            
            for cmd in "${verify_commands[@]}"; do
                if ! eval "$cmd"; then
                    sudo_ok=false
                    break
                fi
            done
            
            if [ "$sudo_ok" = false ]; then
                log_error "当前用户($INSTALL_USER)没有sudo权限，请执行以下步骤后重试：

1. 联系系统管理员将当前用户添加到sudo组：
   sudo usermod -aG sudo $INSTALL_USER

2. 或者让管理员在/etc/sudoers.d/目录下创建配置文件（推荐）：
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   Defaults:$INSTALL_USER !authenticate
   $INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL
   EOF
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER

3. 验证sudo权限（以下命令都应该成功执行）：
   sudo whoami             # 应该输出 root
   sudo ls /root           # 应该能访问root目录
   sudo systemctl status   # 应该能执行系统管理命令

注意：
- 建议使用方式2配置sudo权限，这样无需输入密码
- 确保授予的权限足够安装和管理服务
- 添加权限后需要重新登录生效"
                return 1
            fi
        fi
        return 0
    }
    
    # 检查所有节点的sudo权限
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        for node in "${ALL_NODES[@]}"; do
            if [ "$node" != "localhost" ] && [ "$node" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_sudo_permission "$node" true; then
                    exit 1
                fi
            else
                if ! check_sudo_permission "$node" false; then
                    exit 1
                fi
            fi
        done
    else
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" != "localhost" ] && [ "$master" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_sudo_permission "$master" true; then
                    exit 1
                fi
            else
                if ! check_sudo_permission "$master" false; then
                    exit 1
                fi
            fi
        done
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" != "localhost" ] && [ "$worker" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_sudo_permission "$worker" true; then
                    exit 1
                fi
            else
                if ! check_sudo_permission "$worker" false; then
                    exit 1
                fi
            fi
        done
    fi
    
    # 检查指定用户是否存在
    if ! id "$INSTALL_USER" >/dev/null 2>&1; then
        log_error "安装用户($INSTALL_USER)不存在，请按以下步骤操作：

1. 创建用户和用户组：
   sudo groupadd $INSTALL_GROUP
   sudo useradd -m -g $INSTALL_GROUP $INSTALL_USER

2. 设置用户密码：
   sudo passwd $INSTALL_USER

3. 配置sudo权限（选择以下任一方式）：

   方式1：将用户添加到sudo组（需要输入密码）
   sudo usermod -aG sudo $INSTALL_USER

   方式2：创建sudo权限配置文件（推荐，无需输入密码）
   sudo tee /etc/sudoers.d/$INSTALL_USER << EOF
   Defaults:$INSTALL_USER !authenticate
   $INSTALL_USER ALL=(ALL:ALL) NOPASSWD: ALL
   EOF
   sudo chmod 440 /etc/sudoers.d/$INSTALL_USER

4. 切换到新用户并验证：
   su - $INSTALL_USER
   sudo whoami  # 应该输出 root

5. 重新运行安装脚本：
   ./install_seatunnel.sh

注意：
- 建议使用方式2配置sudo权限，这样无需输入密码
- 如果使用方式1，需要确保系统中存在sudo组
- 某些系统中sudo组可能叫wheel组
- 添加sudo权限后需要重新登录才能生效
- 如果是多节点安装，需要在所有节点上执行上述步骤"
        exit 1
    fi
    
    # 检查用户组是否存在
    if ! getent group "$INSTALL_GROUP" >/dev/null; then
        log_error "用户组($INSTALL_GROUP)不存在，请执行以下命令创建用户组后重试：

1. 创建用户组：
   sudo groupadd $INSTALL_GROUP

2. 将用户添加到用户组：
   sudo usermod -aG $INSTALL_GROUP $INSTALL_USER"
        exit 1
    fi

    # 检查当前用户是否有权限访问安装目录
    if [ ! -d "$BASE_DIR" ]; then
        # 如果目录不存在，检查是否有权限创建
        if ! mkdir -p "$BASE_DIR" 2>/dev/null; then
            log_error "当前用户($INSTALL_USER)无法创建安装目录($BASE_DIR)，请执行以下命令后重试：

sudo mkdir -p $BASE_DIR
sudo chown -R $INSTALL_USER:$INSTALL_GROUP $BASE_DIR"
            exit 1
        fi
    elif [ ! -w "$BASE_DIR" ]; then
        # 如果目录存在但没有写权限
        log_error "当前用户($INSTALL_USER)没有安装目录($BASE_DIR)的写入权限，请执行以下命令后重试：

sudo chown -R $INSTALL_USER:$INSTALL_GROUP $BASE_DIR"
        exit 1
    fi
}

# SeaTunnel版本检测 (依赖util.sh中的version_ge)
is_seatunnel_ge_239() {
    version_ge "${SEATUNNEL_VERSION:-0}" "2.3.9"
}

# ============================================================================
# 安装步骤定义
# ============================================================================
# 每个步骤的名称和描述
declare -A INSTALL_STEPS=(
    [1]="read_config:读取配置文件"
    [2]="check_user:检查用户配置"
    [3]="check_firewall:检查防火墙状态"
    [4]="check_java:检查Java环境"
    [5]="check_dependencies:检查系统依赖"
    [6]="check_memory:检查系统内存"
    [7]="check_ports:检查端口占用"
    [8]="handle_package:处理安装包"
    [9]="setup_config:配置集群模式"
    [10]="configure_checkpoint:配置检查点存储"
    [11]="install_plugins:安装插件和依赖"
    [12]="distribute_nodes:分发到其他节点"
    [13]="setup_environment:配置环境变量"
    [14]="setup_auto_start:配置开机自启动"
    [15]="start_cluster:启动集群"
    [16]="check_services:检查服务状态"
)
TOTAL_STEPS=${#INSTALL_STEPS[@]}

# 当前步骤状态文件
STEP_STATUS_FILE="$EXEC_PATH/.install_step_status"

# 输出模式: cli / json
OUTPUT_MODE="cli"

# 默认使用自动选择模式
FORCE_SED=false
ONLY_INSTALL_PLUGINS=false
NO_PLUGINS=false
RUN_STEP=""           # 指定运行的步骤
STOP_AT_STEP=""       # 停止在指定步骤
LIST_STEPS=false      # 列出所有步骤
WEB_MODE=false        # Web模式

# 显示帮助信息 (必须在参数解析之前定义)
show_help() {
    cat << 'EOF'
SeaTunnel 安装脚本

用法: ./install_seatunnel.sh [选项]

选项:
    --list-steps        列出所有安装步骤
    --step <N>          仅执行指定步骤 (1-16)
    --stop-at <N>       执行到指定步骤后停止
    --json              输出 JSON 格式 (用于 Web 模式)
    --install-plugins   仅安装/更新插件
    --no-plugins        不安装插件
    --force-sed         强制使用 sed 修改配置
    --help, -h          显示此帮助信息

示例:
    ./install_seatunnel.sh                    # 完整安装
    ./install_seatunnel.sh --list-steps       # 查看所有步骤
    ./install_seatunnel.sh --stop-at 7        # 执行到端口检查后停止
    ./install_seatunnel.sh --step 8           # 仅执行安装包处理
    ./install_seatunnel.sh --json --step 1    # JSON格式输出执行步骤1
EOF
}

# 解析命令行参数
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --force-sed) FORCE_SED=true ;;
        --install-plugins) ONLY_INSTALL_PLUGINS=true ;;
        --no-plugins) NO_PLUGINS=true ;;
        --step) RUN_STEP="$2"; shift ;;
        --stop-at) STOP_AT_STEP="$2"; shift ;;
        --list-steps) LIST_STEPS=true ;;
        --json) OUTPUT_MODE="json" ;;
        --help|-h) show_help; exit 0 ;;
        *) log_error "未知参数: $1" ;;
    esac
    shift
done

# 列出所有步骤
list_all_steps() {
    if [ "$OUTPUT_MODE" = "json" ]; then
        echo '{"steps":['
        local first=true
        for i in $(seq 1 $TOTAL_STEPS); do
            local step_info="${INSTALL_STEPS[$i]}"
            local step_func="${step_info%%:*}"
            local step_desc="${step_info#*:}"
            [ "$first" = true ] && first=false || echo ','
            echo "{\"step\":$i,\"name\":\"$step_func\",\"description\":\"$step_desc\"}"
        done
        echo ']}'
    else
        echo ""
        echo "SeaTunnel 安装步骤:"
        echo "============================================"
        for i in $(seq 1 $TOTAL_STEPS); do
            local step_info="${INSTALL_STEPS[$i]}"
            local step_func="${step_info%%:*}"
            local step_desc="${step_info#*:}"
            printf "  %2d. %-25s %s\n" "$i" "$step_func" "$step_desc"
        done
        echo ""
        echo "用法: ./install_seatunnel.sh --step <N>    执行指定步骤"
        echo "      ./install_seatunnel.sh --stop-at <N> 执行到指定步骤停止"
        echo ""
    fi
}

# JSON 输出函数
json_output() {
    local status="$1"
    local step="$2"
    local message="$3"
    local extra="${4:-}"
    
    if [ "$OUTPUT_MODE" = "json" ]; then
        local json="{\"status\":\"$status\",\"step\":$step,\"message\":\"$message\""
        [ -n "$extra" ] && json+=",$extra"
        json+="}"
        echo "$json"
    fi
}

# 保存步骤状态
save_step_status() {
    local step="$1"
    local status="$2"
    echo "$step:$status" >> "$STEP_STATUS_FILE"
}

# 获取步骤状态
get_step_status() {
    local step="$1"
    if [ -f "$STEP_STATUS_FILE" ]; then
        grep "^$step:" "$STEP_STATUS_FILE" | tail -1 | cut -d':' -f2
    fi
}

# 清除步骤状态
clear_step_status() {
    rm -f "$STEP_STATUS_FILE"
}

# 修改check_command函数
check_command() {
    local cmd=$1
    if [ "$cmd" = "awk" ]; then
        # 检查awk是否可用
        command -v "$cmd" >/dev/null 2>&1
    else
        # 其他命令强制使用指定的模式
        if [ "$FORCE_SED" = true ]; then
            return 1
        fi
        command -v "$cmd" >/dev/null 2>&1
    fi
}

# 更新replace_yaml_section函数
replace_yaml_section() {
    local file=$1        # yaml文件路径
    local section=$2     # 要替换的部分的开始标记（如 member-list:, plugin-config:）
    local indent=$3      # 新内容的缩进空格数
    local content=$4     # 新的内容
    local temp_file
    
    log_info "修改配置文件: $file, 替换部分: $section"
    
    # 创建临时文件
    temp_file=$(create_temp_file)
    
    if [ "$FORCE_SED" = true ]; then
        log_info "强制使用sed处理文件..."
        # 获取section的缩进和完整行
        local section_line
        section_line=$(grep "$section" "$file")
        local section_indent
        section_indent=$(echo "$section_line" | sed 's/[^[:space:]].*//' | wc -c)
        
        # 预处理内容，添加缩进
        local indented_content
        indented_content=$(echo "$content" | sed "s/^/$(printf '%*s' "$indent" '')/")
        
        # 创建临时文件存储新内容
        local content_file=$(create_temp_file)
        echo "$indented_content" > "$content_file"
        
        # 第一步：找section的起始行号
        local start_line
        start_line=$(grep -n "$section" "$file" | cut -d: -f1)
        
        # 第二步：找到section的结束行号
        local end_line
        end_line=$(tail -n +$((start_line + 1)) "$file" | grep -n "^[[:space:]]\{0,$section_indent\}[^[:space:]]" | head -1 | cut -d: -f1)
        end_line=$((start_line + end_line))
        
        # 第三步：组合新文件
        # 1. 复制section之前的内容
        sed -n "1,${start_line}p" "$file" > "$temp_file"
        # 2. 添加新内容
        cat "$content_file" >> "$temp_file"
        # 3. 复制section之后的内容
        sed -n "$((end_line)),\$p" "$file" >> "$temp_file"
        
        # 清理临时内容文件
        rm -f "$content_file"
    else
        log_info "使用awk处理文件..."
        # awk版本的实现
        awk -v section="$section" -v base_indent="$indent" -v content="$content" '
        # 计算行的缩进空格数
        function get_indent(line) {
            match(line, /^[[:space:]]*/)
            return RLENGTH
        }
        
        # 为每行添加缩进的函数
        function add_indent(str, indent,    lines, i, result) {
            split(str, lines, "\n")
            result = ""
            for (i = 1; i <= length(lines); i++) {
                if (lines[i] != "") {
                    result = result sprintf("%*s%s\n", indent, "", lines[i])
                }
            }
            return result
        }
        
        BEGIN { 
            in_section = 0
            section_indent = -1
            # 预处理content，添加缩进
            indented_content = add_indent(content, base_indent)
        }
        {
            current_indent = get_indent($0)
            
            if ($0 ~ section) {
                # 找到section，记录其缩进级别
                section_indent = current_indent
                print $0
                printf "%s", indented_content
                in_section = 1
                next
            }
            
            if (in_section) {
                # 如果当前行的缩进小于等section的缩进，说明section结束
                if (current_indent <= section_indent && $0 !~ "^[[:space:]]*$") {
                    in_section = 0
                    section_indent = -1
                    print $0
                }
            } else {
                print $0
            }
        }' "$file" > "$temp_file"
    fi
    
    # 获取文件权限，使用ls -l作为备选方案
    local file_perms
    if stat --version 2>/dev/null | grep -q 'GNU coreutils'; then
        # GNU stat
        file_perms=$(stat -c %a "$file")
    else
        # 其他系统，使用ls -l解析
        file_perms=$(ls -l "$file" | cut -d ' ' -f1 | tr 'rwx-' '7500' | sed 's/^.\(.*\)/\1/' | tr -d '\n')
    fi
    
    # 复制新内容到原文件
    cp "$temp_file" "$file"
    
    # 恢复文件权限
    chmod "$file_perms" "$file"
}

# 修改hazelcast配置文件
modify_hazelcast_config() {
    local config_file=$1
    
    # 备份文件
    cp "$config_file" "${config_file}.bak"
    
    case "$config_file" in
        *"hazelcast.yaml")
            log_info "修改 hazelcast.yaml (集群通信配置)..."
            if [ "$DEPLOY_MODE" = "hybrid" ]; then
                # 构建member-list数组
                local members_json="["
                for node in "${ALL_NODES[@]}"; do
                    members_json+="\"${node}:${HYBRID_PORT:-5801}\","
                done
                members_json="${members_json%,}]"  # 移除最后的逗号
                
                # 使用yq修改member-list和port
                replace_yaml_with_yq "$config_file" \
                    ".hazelcast.network.join.\"tcp-ip\".\"member-list\" = $members_json | .hazelcast.network.port.port = ${HYBRID_PORT:-5801}"
            fi
            ;;
        *"hazelcast-client.yaml")
            log_info "修改 hazelcast-client.yaml (客户端连接配置)..."
            # 生成cluster-members数组
            local members_json="["
            if [ "$DEPLOY_MODE" = "hybrid" ]; then
                log_info "混合模式: 客户端可连接任意节点的 ${HYBRID_PORT:-5801} 端口"
                for node in "${ALL_NODES[@]}"; do
                    members_json+="\"${node}:${HYBRID_PORT:-5801}\","
                done
            else
                log_info "分离模式: 客户端仅连接Master节点的 ${MASTER_PORT:-5801} 端口"
                for master in "${MASTER_IPS[@]}"; do
                    members_json+="\"${master}:${MASTER_PORT:-5801}\","
                done
            fi
            members_json="${members_json%,}]"  # 移除最后的逗号
            
            # 使用yq修改cluster-members
            replace_yaml_with_yq "$config_file" \
                ".\"hazelcast-client\".network.\"cluster-members\" = $members_json"
            ;;
        *"hazelcast-master.yaml")
            if [ "$DEPLOY_MODE" != "hybrid" ]; then
                log_info "修改 hazelcast-master.yaml (Master节点配置)..."
                log_info "分离模式: Master使用 ${MASTER_PORT:-5801} 端口，Worker使用 ${WORKER_PORT:-5802} 端口"
                
                # 构建member-list数组（包含master和worker）
                local members_json="["
                for master in "${MASTER_IPS[@]}"; do
                    members_json+="\"${master}:${MASTER_PORT:-5801}\","
                done
                for worker in "${WORKER_IPS[@]}"; do
                    members_json+="\"${worker}:${WORKER_PORT:-5802}\","
                done
                members_json="${members_json%,}]"  # 移除最后的逗号
                
                # 使用yq修改member-list和port
                replace_yaml_with_yq "$config_file" \
                    ".hazelcast.network.join.\"tcp-ip\".\"member-list\" = $members_json | .hazelcast.network.port.port = ${MASTER_PORT:-5801}"
            fi
            ;;
        *"hazelcast-worker.yaml")
            if [ "$DEPLOY_MODE" != "hybrid" ]; then
                log_info "修改 hazelcast-worker.yaml (Worker节点配置)..."
                log_info "分离模式: Master使用 ${MASTER_PORT:-5801} 端口，Worker使用 ${WORKER_PORT:-5802} 端口"
                
                # 构建member-list数组（包含master和worker）
                local members_json="["
                for master in "${MASTER_IPS[@]}"; do
                    members_json+="\"${master}:${MASTER_PORT:-5801}\","
                done
                for worker in "${WORKER_IPS[@]}"; do
                    members_json+="\"${worker}:${WORKER_PORT:-5802}\","
                done
                members_json="${members_json%,}]"  # 移除最后的逗号
                
                # 使用yq修改member-list和port
                replace_yaml_with_yq "$config_file" \
                    ".hazelcast.network.join.\"tcp-ip\".\"member-list\" = $members_json | .hazelcast.network.port.port = ${WORKER_PORT:-5802}"
            fi
            ;;
    esac
}

# 配置混合模式
setup_hybrid_mode() {
    log_info "配置混合模式集群..."
    
    # 配置hazelcast.yaml，所有节点使用5801端口
    modify_hazelcast_config "$SEATUNNEL_HOME/config/hazelcast.yaml"
    
    # 配置client，所有点都可以作为连接点
    modify_hazelcast_config "$SEATUNNEL_HOME/config/hazelcast-client.yaml"
    
    # 配置JVM选项
    configure_jvm_options "$SEATUNNEL_HOME/config/jvm_options" "$HYBRID_HEAP_SIZE"
}

# 配置分离模式
setup_separated_mode() {
    log_info "配置分离模式集群..."
    
    # 配置master节点
    modify_hazelcast_config "$SEATUNNEL_HOME/config/hazelcast-master.yaml"
    
    # 配置worker节点
    modify_hazelcast_config "$SEATUNNEL_HOME/config/hazelcast-worker.yaml"
    
    # 配置client
    modify_hazelcast_config "$SEATUNNEL_HOME/config/hazelcast-client.yaml"
    
    # 配置JVM选项
    configure_jvm_options "$SEATUNNEL_HOME/config/jvm_master_options" "$MASTER_HEAP_SIZE"
    configure_jvm_options "$SEATUNNEL_HOME/config/jvm_worker_options" "$WORKER_HEAP_SIZE"
}

# 启动集群
start_cluster() {
    log_info "启动SeaTunnel集群..."
    
    if [ "${ENABLE_AUTO_START}" = "true" ]; then
        # 使用systemd服务启动
        local current_ip
        current_ip=$(hostname -I | awk '{print $1}')
        
        if [ "$DEPLOY_MODE" = "hybrid" ]; then
            # 混合模式：在所有节点上启动服务
            for node in "${ALL_NODES[@]}"; do
                if [ "$node" = "localhost" ] || [ "$node" = "$current_ip" ]; then
                    log_info "在本地节点启动服务..."
                    if ! sudo systemctl start seatunnel; then
                        log_error "启动本地服务失败，请手动执行：
sudo systemctl start seatunnel
sudo systemctl status seatunnel  # 查看状态"
                        return 1
                    fi
                    continue
                fi
                
                log_info "在节点 $node 上启动服务..."
                if ! ssh_with_retry "$node" "sudo systemctl start seatunnel"; then
                    log_error "启动节点 $node 的服务失败，请手动在该节点执行：
sudo systemctl start seatunnel
sudo systemctl status seatunnel  # 查看状态"
                    return 1
                fi
            done
        else
            # 分离模式：根据节点角色启动对应服务
            # 启动Master节点
            for master in "${MASTER_IPS[@]}"; do
                if [ "$master" = "localhost" ] || [ "$master" = "$current_ip" ]; then
                    log_info "在本地Master节点启动服务..."
                    if ! sudo systemctl start seatunnel-master; then
                        log_error "启动本地Master服务失败，请手动执行：
sudo systemctl start seatunnel-master
sudo systemctl status seatunnel-master  # 查看状态"
                        return 1
                    fi
                    continue
                fi
                
                log_info "在Master节点 $master 上启动服务..."
                if ! ssh_with_retry "$master" "sudo systemctl start seatunnel-master"; then
                    log_error "启动Master节点 $master 的服务失败，请手动在该节点执行：
sudo systemctl start seatunnel-master
sudo systemctl status seatunnel-master  # 查看状态"
                    return 1
                fi
            done
            
            # 启动Worker节点
            for worker in "${WORKER_IPS[@]}"; do
                if [ "$worker" = "localhost" ] || [ "$worker" = "$current_ip" ]; then
                    log_info "在本地Worker节点启动服务..."
                    if ! sudo systemctl start seatunnel-worker; then
                        log_error "启动本地Worker服务失败，请手动执行：
sudo systemctl start seatunnel-worker
sudo systemctl status seatunnel-worker  # 查看状态"
                        return 1
                    fi
                    continue
                fi
                
                log_info "在Worker节点 $worker 上启动服务..."
                if ! ssh_with_retry "$worker" "sudo systemctl start seatunnel-worker"; then
                    log_error "启动Worker节点 $worker 的服务失败，请手动在该节点执行：
sudo systemctl start seatunnel-worker
sudo systemctl status seatunnel-worker  # 查看状态"
                    return 1
                fi
            done
        fi
    else
        # 使用脚本启动
        if ! sudo chmod +x "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh"; then
            log_error "设置启动脚本权限失败，请手动执行：
sudo chmod +x $SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh"
            return 1
        fi
        
        if ! run_as_user "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh start"; then
            log_error "启动集群失败，请检查日志并手动启动：
$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh start"
            return 1
        fi
    fi
}

# 配置JVM选项
configure_jvm_options() {
    local file=$1
    local heap_size=$2
    
    log_info "配置JVM选项: $file (堆内存: ${heap_size}g)"
    
    # 备份原始文件
    cp "$file" "${file}.bak"
    
    # SeaTunnel 2.3.9+ 的 jvm_options 文件中 -Xms/-Xmx 默认是注释状态: # -Xms2g
    # 需要先去掉注释，再修改内存值
    if is_seatunnel_ge_239; then
        # 去掉 # -Xms 和 # -Xmx 行的注释符号
        sed -i 's/^#[[:space:]]*\(-Xms[0-9]\+g\)/\1/' "$file"
        sed -i 's/^#[[:space:]]*\(-Xmx[0-9]\+g\)/\1/' "$file"
    fi
    
    # 修改JVM堆内存配置
    sed -i "s/-Xms[0-9]\+g/-Xms${heap_size}g/" "$file"
    sed -i "s/-Xmx[0-9]\+g/-Xmx${heap_size}g/" "$file"
}

# 检查端口占用
check_ports() {
    log_info "检查端口占用..."
    local occupied_ports=()
    
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        local service_port=${HYBRID_PORT:-5801}
        local http_port=${MASTER_HTTP_PORT:-8080}
        for node in "${ALL_NODES[@]}"; do
            if ! check_port "$node" "$service_port" 2>/dev/null; then
                occupied_ports+=("$node:$service_port")
            fi
            if is_seatunnel_ge_239; then
                if ! check_port "$node" "$http_port" 2>/dev/null; then
                    occupied_ports+=("$node:$http_port(HTTP)")
                fi
            fi
        done
    else
        local master_port=${MASTER_PORT:-5801}
        local worker_port=${WORKER_PORT:-5802}
        local master_http_port=${MASTER_HTTP_PORT:-8080}
        
        # 检查Master节点端口
        for master in "${MASTER_IPS[@]}"; do
            if ! check_port "$master" "$master_port" 2>/dev/null; then
                occupied_ports+=("$master:$master_port")
            fi
            # 检查Master HTTP API端口 (SeaTunnel 2.3.9+)
            if is_seatunnel_ge_239; then
                if ! check_port "$master" "$master_http_port" 2>/dev/null; then
                    occupied_ports+=("$master:$master_http_port(HTTP)")
                fi
            fi
        done
        
        # 检查Worker节点端口
        for worker in "${WORKER_IPS[@]}"; do
            if ! check_port "$worker" "$worker_port" 2>/dev/null; then
                occupied_ports+=("$worker:$worker_port")
            fi
        done
    fi
    
    if [ ${#occupied_ports[@]} -gt 0 ]; then
        log_error "以下端口已被占用:\n${occupied_ports[*]}"
    fi
}

# 检查服务状态
check_services() {
    log_info "检查服务状态..."
    
    # 等待服务启动
    log_info "等待服务启动（10秒）..."
    sleep 10
    
    # 检查所有节点的服务状态
    local nodes=()
    local ports=()
    
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式：所有节点使用相同端口
        nodes=("${ALL_NODES[@]}")
        for node in "${nodes[@]}"; do
            ports+=("${HYBRID_PORT:-5801}")
        done
    else
        # 分离模式：收集所有节点和对应端口
        for master in "${MASTER_IPS[@]}"; do
            nodes+=("$master")
            ports+=("${MASTER_PORT:-5801}")
        done
        for worker in "${WORKER_IPS[@]}"; do
            nodes+=("$worker")
            ports+=("${WORKER_PORT:-5802}")
        done
    fi
    
    # 检查每个节点的服务状态
    local success=true
    for i in "${!nodes[@]}"; do
        local node="${nodes[$i]}"
        local port="${ports[$i]}"
        
        log_info "检查节点 $node:$port 的服务状态..."
        
        # 处理localhost的情况
        if [ "$node" = "localhost" ]; then
            node="127.0.0.1"
        fi
        
        # 尝试多种方式检查端口
        local service_running=false
        
        if command -v nc >/dev/null 2>&1; then
            # 使用nc命令检查
            log_info "使用nc命令检查节点 $node:$port 的服务状态..."
            if [ "$node" = "127.0.0.1" ] || [ "$node" = "$(hostname -I | awk '{print $1}')" ]; then
                # 本地检查使用localhost
                if nc -z -w2 localhost "$port" >/dev/null 2>&1; then
                    service_running=true
                fi
            else
                # 远程节点检查
                if nc -z -w2 "$node" "$port" >/dev/null 2>&1; then
                    service_running=true
                fi
            fi
        else
            # 使用/dev/tcp
            log_info "使用/dev/tcp命令检查节点 $node:$port 的服务状态..."
            if [ "$node" = "127.0.0.1" ] || [ "$node" = "$(hostname -I | awk '{print $1}')" ]; then
                # 本地检查使用localhost
                if timeout 2 bash -c "echo >/dev/tcp/localhost/$port" >/dev/null 2>&1; then
                    service_running=true
                fi
            else
                # 远程节点检查
                if timeout 2 bash -c "echo >/dev/tcp/$node/$port" >/dev/null 2>&1; then
                    service_running=true
                fi
            fi
        fi
        
        if [ "$service_running" = true ]; then
            log_success "节点 $node:$port 服务运行正常"
        else
            log_warning "节点 $node:$port 服务未响应"
            success=false
        fi
    done
    
    if [ "$success" = true ]; then
        log_success "所有节点服务检查通过"
    else
        log_warning "部分节点服务检查未通过，请检查日志确认具体原因"
    fi
    
    log_success "所有服务运行正常"
}

# 配置检查点存储
configure_checkpoint() {
    # 计算实际节点数（排除localhost）
    local actual_node_count=0
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        for node in "${ALL_NODES[@]}"; do
            if [ "$node" != "localhost" ]; then
                actual_node_count=$((actual_node_count + 1))
            fi
        done
    else
        # 分离模式：计算master和worker节点总数
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" != "localhost" ]; then
                actual_node_count=$((actual_node_count + 1))
            fi
        done
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" != "localhost" ]; then
                actual_node_count=$((actual_node_count + 1))
            fi
        done
    fi
    
    local content
    
    # Validate storage type
    if [[ -z "$CHECKPOINT_STORAGE_TYPE" ]]; then
        log_info "未配置检查点存储类��，使��默认配置"
        CHECKPOINT_STORAGE_TYPE="LOCAL_FILE"
    fi

    # Validate required variables based on storage type
    case "$CHECKPOINT_STORAGE_TYPE" in
        LOCAL_FILE)
            [[ -z "$CHECKPOINT_NAMESPACE" ]] && log_error "LOCAL_FILE 模式需要配置 CHECKPOINT_NAMESPACE"
            ;;
        HDFS)
            [[ -z "$CHECKPOINT_NAMESPACE" ]] && log_error "HDFS 模式需要配置 CHECKPOINT_NAMESPACE"
            [[ -z "$HDFS_NAMENODE_HOST" ]] && log_error "HDFS 模式需要配置 HDFS_NAMENODE_HOST"
            [[ -z "$HDFS_NAMENODE_PORT" ]] && log_error "HDFS 模式需要配置 HDFS_NAMENODE_PORT"
            ;;
        OSS|S3)
            [[ -z "$CHECKPOINT_NAMESPACE" ]] && log_error "${CHECKPOINT_STORAGE_TYPE} 模式需要配置 CHECKPOINT_NAMESPACE"
            [[ -z "$STORAGE_BUCKET" ]] && log_error "${CHECKPOINT_STORAGE_TYPE} 模式需要配置 STORAGE_BUCKET"
            [[ -z "$STORAGE_ENDPOINT" ]] && log_error "${CHECKPOINT_STORAGE_TYPE} 模式需要配置 STORAGE_ENDPOINT"
            [[ -z "$STORAGE_ACCESS_KEY" ]] && log_error "${CHECKPOINT_STORAGE_TYPE} 模式需要配置 STORAGE_ACCESS_KEY"
            [[ -z "$STORAGE_SECRET_KEY" ]] && log_error "${CHECKPOINT_STORAGE_TYPE} 模式需要配置 STORAGE_SECRET_KEY"
            ;;
        *)
            log_error "不支持的检查点存储类型: $CHECKPOINT_STORAGE_TYPE"
            ;;
    esac
    
    # 如果是LOCAL_FILE类型，创建本地目录
    if [[ "$CHECKPOINT_STORAGE_TYPE" == "LOCAL_FILE" ]]; then
        local checkpoint_dir="$SEATUNNEL_HOME/checkpoint"
        create_directory "$checkpoint_dir"
        setup_permissions "$checkpoint_dir"
        CHECKPOINT_NAMESPACE="$checkpoint_dir"
        
        # 在其他节点上创建目录（排除localhost）
        for node in "${ALL_NODES[@]}"; do
            # 跳过localhost和当前节点
            if [ "$node" = "localhost" ] || [ "$node" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地节点: $node"
                continue
            fi
            
            log_info "在节点 $node 上创建检查点目录..."
            ssh_with_retry "$node" "mkdir -p $checkpoint_dir && chown $INSTALL_USER:$INSTALL_GROUP $checkpoint_dir && chmod 755 $checkpoint_dir"
        done
        
        # 只有在实际节点数大于1时才显示警告
        if [ "$actual_node_count" -gt 1 ]; then
            log_warning "检测到多节点部署，不建议使用本地文件存储作检查点。建议使用 HDFS、OSS 或 S3。"
        fi
    fi
    
    # 根据存储类型生成配置内容
    case "$CHECKPOINT_STORAGE_TYPE" in
        LOCAL_FILE)
            content="namespace: ${CHECKPOINT_NAMESPACE}
storage.type: local"
            ;;
        HDFS)
            content="namespace: ${CHECKPOINT_NAMESPACE}
storage.type: hdfs
fs.defaultFS: hdfs://${HDFS_NAMENODE_HOST}:${HDFS_NAMENODE_PORT}"
            if [ ! -z "${KERBEROS_PRINCIPAL:-}" ] && [ ! -z "${KERBEROS_KEYTAB:-}" ]; then
                content+="
kerberosPrincipal: ${KERBEROS_PRINCIPAL}
kerberosKeytabFilePath: ${KERBEROS_KEYTAB}"
            fi
            ;;
        OSS)
            content="namespace: ${CHECKPOINT_NAMESPACE}
storage.type: oss
oss.bucket: ${STORAGE_BUCKET}
fs.oss.endpoint: ${STORAGE_ENDPOINT}
fs.oss.accessKeyId: ${STORAGE_ACCESS_KEY}
fs.oss.accessKeySecret: ${STORAGE_SECRET_KEY}"
            ;;
        S3)
            content="namespace: ${CHECKPOINT_NAMESPACE}
storage.type: s3
s3.bucket: ${STORAGE_BUCKET}
fs.s3a.endpoint: ${STORAGE_ENDPOINT}
fs.s3a.access.key: ${STORAGE_ACCESS_KEY}
fs.s3a.secret.key: ${STORAGE_SECRET_KEY}
fs.s3a.aws.credentials.provider: org.apache.hadoop.fs.s3a.SimpleAWSCredentialsProvider
disable.cache: true"
            ;;
    esac
    
    # 使用yq修改 seatunnel.engine.checkpoint.storage.plugin-config
    local seatunnel_yaml="$SEATUNNEL_HOME/config/seatunnel.yaml"
    case "$CHECKPOINT_STORAGE_TYPE" in
        LOCAL_FILE)
            replace_yaml_with_yq "$seatunnel_yaml" \
                '.seatunnel.engine.checkpoint.storage."plugin-config" = {"namespace": env(CHECKPOINT_NAMESPACE), "storage.type": "local"}' \
                "CHECKPOINT_NAMESPACE='$CHECKPOINT_NAMESPACE'"
            ;;
        HDFS)
            replace_yaml_with_yq "$seatunnel_yaml" \
                '.seatunnel.engine.checkpoint.storage."plugin-config" = {"namespace": env(CHECKPOINT_NAMESPACE), "storage.type": "hdfs", "fs.defaultFS": ("hdfs://" + env(HDFS_NAMENODE_HOST) + ":" + env(HDFS_NAMENODE_PORT))}' \
                "CHECKPOINT_NAMESPACE='$CHECKPOINT_NAMESPACE' HDFS_NAMENODE_HOST='$HDFS_NAMENODE_HOST' HDFS_NAMENODE_PORT='$HDFS_NAMENODE_PORT'"
            ;;
        OSS)
            replace_yaml_with_yq "$seatunnel_yaml" \
                '.seatunnel.engine.checkpoint.storage."plugin-config" = {"namespace": env(CHECKPOINT_NAMESPACE), "storage.type": "oss", "oss.bucket": env(STORAGE_BUCKET), "fs.oss.endpoint": env(STORAGE_ENDPOINT), "fs.oss.accessKeyId": env(STORAGE_ACCESS_KEY), "fs.oss.accessKeySecret": env(STORAGE_SECRET_KEY)}' \
                "CHECKPOINT_NAMESPACE='$CHECKPOINT_NAMESPACE' STORAGE_BUCKET='$STORAGE_BUCKET' STORAGE_ENDPOINT='$STORAGE_ENDPOINT' STORAGE_ACCESS_KEY='$STORAGE_ACCESS_KEY' STORAGE_SECRET_KEY='$STORAGE_SECRET_KEY'"
            ;;
        S3)
            replace_yaml_with_yq "$seatunnel_yaml" \
                '.seatunnel.engine.checkpoint.storage."plugin-config" = {"namespace": env(CHECKPOINT_NAMESPACE), "storage.type": "s3", "s3.bucket": env(STORAGE_BUCKET), "fs.s3a.endpoint": env(STORAGE_ENDPOINT), "fs.s3a.access.key": env(STORAGE_ACCESS_KEY), "fs.s3a.secret.key": env(STORAGE_SECRET_KEY), "fs.s3a.aws.credentials.provider": "org.apache.hadoop.fs.s3a.SimpleAWSCredentialsProvider", "disable.cache": true}' \
                "CHECKPOINT_NAMESPACE='$CHECKPOINT_NAMESPACE' STORAGE_BUCKET='$STORAGE_BUCKET' STORAGE_ENDPOINT='$STORAGE_ENDPOINT' STORAGE_ACCESS_KEY='$STORAGE_ACCESS_KEY' STORAGE_SECRET_KEY='$STORAGE_SECRET_KEY'"
            ;;
    esac
}

# 配置Master HTTP API端口 (SeaTunnel 2.3.9+)
configure_master_http_port() {
    local seatunnel_yaml="$SEATUNNEL_HOME/config/seatunnel.yaml"
    local http_port=${MASTER_HTTP_PORT:-8080}
    
    log_info "配置Master HTTP API端口: $http_port"
    
    # 检查seatunnel.yaml是否存在http配置节
    if grep -q "seatunnel:" "$seatunnel_yaml" && grep -q "engine:" "$seatunnel_yaml"; then
        # 使用yq修改HTTP端口配置
        replace_yaml_with_yq "$seatunnel_yaml" \
            '.seatunnel.engine.http."enable-http" = true | .seatunnel.engine.http.port = env(HTTP_PORT) | .seatunnel.engine.http."enable-dynamic-port" = false' \
            "HTTP_PORT='$http_port'"
        
        log_info "Master HTTP API端口已配置为: $http_port"
    else
        log_warning "seatunnel.yaml中未找到engine配置节，跳过HTTP端口配置"
    fi
}

# 获取统一的 JAVA_HOME 路径（需要在 read_config 之后调用）
get_unified_java_home() {
    echo "${BASE_DIR}/java"
}

# 设置统一的 JAVA_HOME 软链接
# 确保所有节点使用相同的 JAVA_HOME 路径，便于生成统一的 systemd 服务文件
setup_java_symlink() {
    local node=$1
    local is_remote=$2
    local unified_java_home
    unified_java_home=$(get_unified_java_home)
    
    log_info "设置节点 $node 的统一 JAVA_HOME: $unified_java_home"
    
    if [ "$is_remote" = "false" ]; then
        # 本地节点
        # 如果统一路径已存在且是目录（不是软链接），说明是之前安装的 Java，跳过
        if [ -d "$unified_java_home" ] && [ ! -L "$unified_java_home" ]; then
            log_info "本地节点 JAVA_HOME 已存在: $unified_java_home"
            return 0
        fi
        
        # 如果统一路径已存在且是软链接，先删除
        if [ -L "$unified_java_home" ]; then
            rm -f "$unified_java_home"
        fi
        
        # 获取实际的 JAVA_HOME
        local real_java_home
        if [ -n "$JAVA_HOME" ] && [ -d "$JAVA_HOME" ]; then
            real_java_home="$JAVA_HOME"
        else
            # 从 java 命令路径推断 JAVA_HOME
            local java_bin
            java_bin=$(which java 2>/dev/null)
            if [ -n "$java_bin" ]; then
                # 解析软链接获取真实路径
                java_bin=$(readlink -f "$java_bin")
                # java 在 bin 目录下，JAVA_HOME 是其父目录的父目录
                real_java_home=$(dirname "$(dirname "$java_bin")")
            fi
        fi
        
        if [ -n "$real_java_home" ] && [ -d "$real_java_home" ]; then
            # 创建软链接
            mkdir -p "$(dirname "$unified_java_home")"
            ln -sf "$real_java_home" "$unified_java_home"
            log_info "本地节点创建 JAVA_HOME 软链接: $unified_java_home -> $real_java_home"
        else
            log_warning "本地节点无法确定 JAVA_HOME，跳过软链接创建"
        fi
    else
        # 远程节点
        # 检查统一路径是否已存在
        local path_exists
        path_exists=$(ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "[ -e '$unified_java_home' ] && echo 'yes' || echo 'no'" 2>/dev/null)
        
        if [ "$path_exists" = "yes" ]; then
            local is_link
            is_link=$(ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "[ -L '$unified_java_home' ] && echo 'yes' || echo 'no'" 2>/dev/null)
            if [ "$is_link" = "no" ]; then
                log_info "远程节点 $node JAVA_HOME 已存在: $unified_java_home"
                return 0
            fi
            # 是软链接，先删除
            ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "rm -f '$unified_java_home'" 2>/dev/null
        fi
        
        # 获取远程节点的实际 JAVA_HOME
        local real_java_home
        real_java_home=$(ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" '
            if [ -n "$JAVA_HOME" ] && [ -d "$JAVA_HOME" ]; then
                echo "$JAVA_HOME"
            else
                java_bin=$(which java 2>/dev/null)
                if [ -n "$java_bin" ]; then
                    java_bin=$(readlink -f "$java_bin")
                    dirname "$(dirname "$java_bin")"
                fi
            fi
        ' 2>/dev/null)
        
        if [ -n "$real_java_home" ]; then
            # 创建软链接
            ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "
                mkdir -p '$(dirname "$unified_java_home")'
                ln -sf '$real_java_home' '$unified_java_home'
            " 2>/dev/null
            log_info "远程节点 $node 创建 JAVA_HOME 软链接: $unified_java_home -> $real_java_home"
        else
            log_warning "远程节点 $node 无法确定 JAVA_HOME，跳过软链接创建"
        fi
    fi
}

# 检查Java环境
check_java() {
    local node=$1
    local is_remote=$2
    
    log_info "检查节点 $node 的Java环境..."
    
    # 本地节点检查
    if [ "$is_remote" = "false" ]; then
        # 检查java命令是否存在
        if ! command -v java >/dev/null 2>&1; then
            log_warning "本地节点未找到Java环境"
            
            if [ "$INSTALL_MODE" != "online" ]; then
                log_error "离线模式下无法自动安装Java，请手动安装Java 8或Java 11"
            fi
            
            # 提示用户选择安装版本
            echo -e "\n${YELLOW}请选择要安装的Java版本:${NC}"
            echo "1) Java 8 (推荐)"
            echo "2) Java 11"
            echo "3) 取消安装"
            
            read -r -p "请输入选项 [1-3]: " choice
            
            case $choice in
                1)
                    install_java "8"
                    ;;
                2)
                    install_java "11"
                    ;;
                3)
                    log_error "用户取消安装"
                    ;;
                *)
                    log_error "无效的选项"
                    ;;
            esac
        fi
        
        # 获取本地Java版本
        local java_version
        java_version=$(java -version 2>&1 | head -n 1 | awk -F '"' '{print $2}')
        if [ -z "$java_version" ]; then
            log_error "无法获取本地Java版本"
        fi
        
        # 检查Java版本是否为8或11
        if [[ $java_version == 1.8* ]]; then
            log_info "节点 $node 检测到Java 8: $java_version"
        elif [[ $java_version == 11* ]]; then
            log_info "节点 $node 检测到Java 11: $java_version"
        else
            log_error "节点 $node 不支持的Java版本: $java_version，SeaTunnel需要Java 8或Java 11"
        fi
        
        # 设置统一的 JAVA_HOME 软链接
        setup_java_symlink "$node" "false"
    else
        # 远程节点检查
        # 添加超时控制
        local TIMEOUT=30
        
        # 先检查java命令是否存在
        if ! timeout $TIMEOUT ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "command -v java" >/dev/null 2>&1; then
            log_warning "节点 $node 未找到Java环境"
            if [ "$INSTALL_MODE" != "online" ]; then
                log_error "离线模式下无法自动安装Java，请在节点 $node 上手动安装Java 8或Java 11"
            fi
            # 在线模式下自动安装Java 8
            log_info "在节点 $node 上自动安装Java 8..."
            install_java "8" "$node"
            return
        fi

        # 获取远程Java版本输出
        local java_version_output
        java_version_output=$(timeout $TIMEOUT ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "java -version 2>&1")
        local exit_code=$?

        # 检查是否超时
        if [ $exit_code = 124 ]; then
            log_error "检查节点 $node 的Java环境超时(${TIMEOUT}秒)"
            return 1
        fi

        # 在本地解析Java版本
        local java_version
        java_version=$(echo "$java_version_output" | head -n 1 | awk -F '"' '{print $2}')

        if [ -z "$java_version" ]; then
            log_error "无法获取节点 $node 的Java版本"
            return 1
        fi

        # 在本地检查版本兼容性
        if [[ $java_version == 1.8* ]]; then
            log_info "节点 $node 检测到Java 8: $java_version"
        elif [[ $java_version == 11* ]]; then
            log_info "节点 $node 检测到Java 11: $java_version"
        else
            log_error "节点 $node 不支持的Java版本: $java_version，SeaTunnel需要Java 8或Java 11"
        fi
        
        # 设置统一的 JAVA_HOME 软链接
        setup_java_symlink "$node" "true"
    fi
}

# 安装Java
install_java() {
    local version=$1
    local node=${2:-"localhost"}
    local java_home="${BASE_DIR}/java"
    local is_remote=false
    
    # 检查是否是远程节点
    if [ "$node" != "localhost" ] && [ "$node" != "$(hostname -I | awk '{print $1}')" ]; then
        is_remote=true
    fi
    
    # 检测系统架构
    local arch
    if [ "$is_remote" = true ]; then
        arch=$(ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "uname -m")
    else
        arch=$(uname -m)
    fi
    
    case "$arch" in
        x86_64)
            arch_suffix="x64"
            ;;
        aarch64)
            arch_suffix="aarch64"
            ;;
        *)
            log_error "不支持的系统架构: $arch"
            ;;
    esac
    
    log_info "开始在节点 $node 上安装Java $version, 系统架构: $arch"
    
    # 构建下载URL和包名
    local download_url
    local java_package
    local java_dir
    
    case $version in
        "8")
            java_package="jdk-8u202-linux-${arch_suffix}.tar.gz"
            java_dir="jdk1.8.0_202"
            download_url="https://repo.huaweicloud.com/java/jdk/8u202-b08/$java_package"
            ;;
        "11")
            java_package="jdk-11.0.2_linux-${arch_suffix}_bin.tar.gz"
            java_dir="jdk-11.0.2"
            download_url="https://repo.huaweicloud.com/java/jdk/11.0.2+9/$java_package"
            ;;
        *)
            log_error "不支持的Java版本: $version"
            ;;
    esac
    
    # 使用全局下载目录
    cd "$DOWNLOAD_DIR" || log_error "无法进入下载目录"
    
    # 检查本地是否已有安装包
    if [ -f "$DOWNLOAD_DIR/$java_package" ]; then
        log_info "发现本地已存在Java安装包: $java_package"
    else
        # 下载Java安装包
        log_info "下载Java安装包..."
        if ! curl -L --progress-bar -o "$java_package" "$download_url"; then
            # 如果华为云下载失败,尝试清华源
            log_warning "从华为云下载失败,尝试清华源..."
            case $version in
                "8")
                    java_package="OpenJDK8U-jdk_${arch_suffix}_linux_hotspot_8u432b06.tar.gz"
                    java_dir="jdk8u432-b06"
                    download_url="https://mirrors.tuna.tsinghua.edu.cn/Adoptium/8/jdk/${arch_suffix}/linux/$java_package"
                    ;;
                "11")
                    java_package="OpenJDK11U-jdk_${arch_suffix}_linux_hotspot_11.0.25_9.tar.gz"
                    java_dir="jdk-11.0.25+9"
                    download_url="https://mirrors.tuna.tsinghua.edu.cn/Adoptium/11/jdk/${arch_suffix}/linux/$java_package"
                    ;;
            esac
            
            if ! curl -L --progress-bar -o "$java_package" "$download_url"; then
                log_error "Java安装包下载失败"
            fi
        fi
    fi
    
    # 创建Java安装目录并解压
    # 解压后创建软链接，使 ${BASE_DIR}/java 直接指向实际的 JDK 目录
    local java_extract_dir="${BASE_DIR}/java_install"
    local java_real_dir="${java_extract_dir}/${java_dir}"
    
    if [ "$is_remote" = true ]; then
        ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "mkdir -p $java_extract_dir"
        scp -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$DOWNLOAD_DIR/$java_package" "${INSTALL_USER}@${node}:$java_extract_dir/"
        ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "
            cd $java_extract_dir && tar -zxf $java_package && rm -f $java_package
            # 创建软链接，使 $java_home 指向实际的 JDK 目录
            rm -f '$java_home' 2>/dev/null || true
            ln -sf '$java_real_dir' '$java_home'
        "
    else
        mkdir -p "$java_extract_dir"
        tar -zxf "$java_package" -C "$java_extract_dir"
        # 创建软链接，使 ${BASE_DIR}/java 指向实际的 JDK 目录
        rm -f "$java_home" 2>/dev/null || true
        ln -sf "$java_real_dir" "$java_home"
    fi
    
    # 设置权限
    if [ "$is_remote" = true ]; then
        ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "chown -R $INSTALL_USER:$INSTALL_GROUP $java_extract_dir && chmod -R 755 $java_extract_dir"
    else
        chown -R "$INSTALL_USER:$INSTALL_GROUP" "$java_extract_dir"
        chmod -R 755 "$java_extract_dir"
    fi
    
    # 配置环境变量（使用统一的 java_home 路径）
    local bashrc_content="
# JAVA_HOME BEGIN
export JAVA_HOME=$java_home
export PATH=\$JAVA_HOME/bin:\$PATH
# JAVA_HOME END"
    
    if [ "$is_remote" = true ]; then
        local remote_home
        remote_home=$(ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "echo ~$INSTALL_USER")
        ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "
            sed -i '/# JAVA_HOME BEGIN/,/# JAVA_HOME END/d' $remote_home/.bashrc
            echo '$bashrc_content' >> $remote_home/.bashrc
            source $remote_home/.bashrc"
    else
        # 获取用户home目录
        local user_home
        if command -v getent >/dev/null 2>&1; then
            user_home=$(getent passwd "$INSTALL_USER" | cut -d: -f6)
        else
            user_home=$(eval echo ~"$INSTALL_USER")
        fi
        
        # 删除已存在的Java配置
        sed -i '/# JAVA_HOME BEGIN/,/# JAVA_HOME END/d' "$user_home/.bashrc"
        echo "$bashrc_content" >> "$user_home/.bashrc"
        
        # 使环境变量生效（使用统一的 java_home 路径）
        export JAVA_HOME="$java_home"
        export PATH="$JAVA_HOME/bin:$PATH"
    fi
    
    # 验证安装
    local verify_cmd="java -version"
    if [ "$is_remote" = true ]; then
        if ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "$verify_cmd" 2>&1 | grep -q "version"; then
            log_success "节点 $node 的Java $version 安装成功"
        else
            log_error "节点 $node 的Java安装失败"
        fi
    else
        if $verify_cmd 2>&1 | grep -q "version"; then
            log_success "本地节点Java $version 安装成功"
        else
            log_error "本地节点Java安装失败"
        fi
    fi
    
    # 不删除安装包,以便重复使用
    log_info "保留Java安装包以供重复使用: $DOWNLOAD_DIR/$java_package"
}


# 添加依赖检查函数
check_dependencies() {
    log_info "检查系统依赖..."
    
    # 必需的命令列表
    local required_cmds=("ssh" "scp" "tar" "grep" "sed")
    
    for cmd in "${required_cmds[@]}"; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            log_error "缺少必需的命令: $cmd"
        fi
    done
    
}

# 检查URL是否可访问
check_url() {
    local url=$1
    local timeout=20
    
    if ! command -v curl >/dev/null 2>&1; then
        log_error "未找到curl命令,请先安装curl"
    fi
    
    if curl --connect-timeout "$timeout" -sI "$url" >/dev/null 2>&1; then
        return 0
    fi
    return 1
}

# 下载安装包
download_package() {
    # 检查是否为在线模式
    if [ "$INSTALL_MODE" != "online" ]; then
        log_error "download_package函数只能在在线模式(INSTALL_MODE=online)下使用"
    fi
    
    local package_name=$1
    local version=$2
    local output_file="$DOWNLOAD_DIR/$package_name"
    local retries=3
    local retry_count=0
    
    log_info "开始下载安装包..."
    
    # 检查curl命令
    if ! command -v curl >/dev/null 2>&1; then
        log_error "未找到curl命令,请先安装curl"
    fi
    
    # 创建并进入下载目录
    mkdir -p "$DOWNLOAD_DIR"
    cd "$DOWNLOAD_DIR" || log_error "无法进入下载目录"
    
    # 获取仓库配置
    local repo=${PACKAGE_REPO:-aliyun}
    local url
    
    # 获取发布包下载地址
    url="${PACKAGE_REPOS[$repo]}"
    if [ -z "$url" ]; then
        log_error "不支持的安装包仓库: $repo"
    fi
    url="$url/${version}/apache-seatunnel-${version}-bin.tar.gz"
    
    log_info "使用下载源: $url"
    
    # 下载重试循环
    while [ $retry_count -lt $retries ]; do
        log_info "下载尝试 $((retry_count + 1))/$retries"
        
        # 检查URL是否可访问
        if ! check_url "$url"; then
            log_warning "当前下载源不可用,尝试切换到备用源..."
            if [ "$repo" = "aliyun" ]; then
                repo="apache"
                url="${PACKAGE_REPOS[$repo]}/${version}/apache-seatunnel-${version}-bin.tar.gz"
                continue
            fi
        fi
        
        # 使用curl下载，显示进度条
        if curl -L \
            --fail \
            --progress-bar \
            --connect-timeout 10 \
            --retry 3 \
            --retry-delay 2 \
            --retry-max-time 60 \
            -o "$output_file" \
            "$url" 2>&1; then
            
            # 验证下载文件
            if [ -f "$output_file" ] && [ -s "$output_file" ]; then
                log_info "下载完成: $output_file"
                echo "$output_file" > /tmp/download_path.tmp
                return 0
            else
                log_warning "下载文件为空或不存在"
            fi
        fi
        
        retry_count=$((retry_count + 1))
        [ $retry_count -lt $retries ] && log_warning "下载失败,等待重试..." && sleep 3
    done
    
    log_error "下载失败,已重试 $retries 次"
    return 1
}

# 添加安装包验证函数
verify_package() {
    local package_file=$1
    
    log_info "验证安装包: $package_file"
    
    # 检查文件是否存在
    if [ ! -f "$package_file" ]; then
        log_error "安装包不存在: $package_file"
    fi
    
    # 检查文件格式
    if ! file "$package_file" | grep -q "gzip compressed data"; then
        log_error "安装包格式错误,必须是tar.gz格式"
    fi
    
    # 检查文件名是否包含版本号
    if ! echo "$package_file" | grep -q "apache-seatunnel-${SEATUNNEL_VERSION}"; then
        log_warning "安装包文件名与配置的版本号不匹配: $SEATUNNEL_VERSION"
        log_warning "安装包: $package_file"
        read -r -p "是否继续安装? [y/N] " response
        case "$response" in
            [yY][eE][sS]|[yY]) 
                log_warning "继续安装..."
                ;;
            *)
                log_error "安装已取消"
                ;;
        esac
    fi
    
    log_info "安装包验证通过"
}

# 添加集群管理脚本
setup_cluster_scripts() {
    log_info "添加集群管理脚本..."
    
    # 获取脚本所在目录的绝对路径
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    # 创建master和workers文件
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        printf "%s\n" "${CLUSTER_NODES[@]}" > "$SEATUNNEL_HOME/bin/master"
        printf "%s\n" "${CLUSTER_NODES[@]}" > "$SEATUNNEL_HOME/bin/workers"
    else
        printf "%s\n" "${MASTER_IPS[@]}" > "$SEATUNNEL_HOME/bin/master"
        printf "%s\n" "${WORKER_IPS[@]}" > "$SEATUNNEL_HOME/bin/workers"
    fi
    
    # 创建集群启动脚本
    cat > "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh" << 'EOF'
#!/bin/bash 
  
# 定义 SeaTunnelServer 进程名称，需要根据实际情况进行修改
PROCESS_NAME="org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer"

# 获取脚本所在目录的绝对路径
bin_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_USER=root

# 定义颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
  echo -e "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

log_error() {
  echo -e "$(date '+%Y-%m-%d %H:%M:%S') [${RED}ERROR${NC}] $1"
}

log_success() {
  echo -e "$(date '+%Y-%m-%d %H:%M:%S') [${GREEN}SUCCESS${NC}] $1"
}

log_warning() {
  echo -e "$(date '+%Y-%m-%d %H:%M:%S') [${YELLOW}WARNING${NC}] $1"
}

export SEATUNNEL_HOME="$(dirname "$bin_dir")"
log_info "SEATUNNEL_HOME: ${SEATUNNEL_HOME}"
master_conf="${bin_dir}/master"
workers_conf="${bin_dir}/workers"

if [ -f "$master_conf" ]; then
    mapfile -t masters < <(sed 's/[[:space:]]*$//' "$master_conf")
else
    log_error "找不到 $master_conf 文件"
    exit 1
fi

if [ -f "$workers_conf" ]; then
    mapfile -t workers < <(sed 's/[[:space:]]*$//' "$workers_conf")
else
    log_error "找不到 $workers_conf 文件"
    exit 1
fi

mapfile -t servers < <(sort -u <(sed 's/[[:space:]]*$//' "$master_conf" "$workers_conf"))

sshPort=22
EOF

    # 继续写入脚本内容...
    cat >> "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh" << 'EOF'

start(){
    echo "-------------------------------------------------"
    for master in "${masters[@]}"; do
        if [ "$master" = "localhost" ]; then
            log_warning "检测到仅有本地进程，跳过远程执行..."
            ${bin_dir}/seatunnel-cluster.sh -d  -r master
            log_success "${master}的SeaTunnel-master启动成功"
        else
            log_info "正在 ${master} 上启动 SeaTunnelServer。"
            ssh -p $sshPort -o StrictHostKeyChecking=no "${INSTALL_USER}@${master}" "source /etc/profile && source ~/.bashrc && ${bin_dir}/seatunnel-cluster.sh -d  -r master"
            log_success "${master}的SeaTunnel-master启动成功"    
        fi
    done

    for worker in "${workers[@]}"; do
        if [ "$worker" = "localhost" ]; then
            log_warning "检测到仅有本地进程，跳过远程执行..."
            ${bin_dir}/seatunnel-cluster.sh -d  -r worker
            log_success "${worker}的SeaTunnel-worker启动成功"
        else
            log_info "正在 ${worker} 上启动 SeaTunnelServer。"
            ssh -p $sshPort -o StrictHostKeyChecking=no "${INSTALL_USER}@${worker}" "source /etc/profile && source ~/.bashrc && ${bin_dir}/seatunnel-cluster.sh -d  -r worker"
            log_success "${worker}的SeaTunnel-worker启动成功"    
        fi
    done
}

stop(){
    echo "-------------------------------------------------"
    for server in "${servers[@]}"; do
        if [ "$server" = "localhost" ]; then
            log_warning "检测到仅有本地进程，跳过远程执行..."
            ${bin_dir}/stop-seatunnel-cluster.sh
            log_success "${server}的SeaTunnel 停止成功"
        else
            log_info "正在 ${server} 上停止 SeaTunnelServer"
            ssh -p $sshPort -o StrictHostKeyChecking=no "${INSTALL_USER}@${server}" "source /etc/profile && source ~/.bashrc && ${bin_dir}/stop-seatunnel-cluster.sh"
            log_success "${server}的SeaTunnel 停止成功"
        fi
    done
}

restart(){
    stop
    sleep 2
    start
}

case "$1" in
    "start")
        start
        ;;
    "stop")
        stop
        ;;
    "restart")
        restart
        ;;
    *)
        echo "用法：$0 {start|stop|restart}"
        exit 1
esac
EOF

    # 设置脚本权限
    chmod +x "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh"
    chmod 644 "$SEATUNNEL_HOME/bin/master"
    chmod 644 "$SEATUNNEL_HOME/bin/workers"
    
    # 设置所有者
    chown "$INSTALL_USER:$INSTALL_GROUP" "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh"
    chown "$INSTALL_USER:$INSTALL_GROUP" "$SEATUNNEL_HOME/bin/master"
    chown "$INSTALL_USER:$INSTALL_GROUP" "$SEATUNNEL_HOME/bin/workers"
    
    log_info "集群管理脚本添加完成"
}

# 安装插件和依赖库
install_plugins_and_libs() {
    # 检查是否需要安装连接器
    if [ "$NO_PLUGINS" = true ] || [ "${INSTALL_CONNECTORS}" != "true" ]; then
        log_info "跳过连接器和依赖安装"
        return 0
    fi

    # 离线模式下跳过插件下载
    if [ "$INSTALL_MODE" = "offline" ]; then
        log_warning "离线安装模式下不支持自动下载插件,如果有需要,请手动将所需插件和依赖放置到以下目录:"
        log_warning "- 插件目录: $SEATUNNEL_HOME/connectors/"
        log_warning "- 依赖目录: $SEATUNNEL_HOME/lib/"
        return 0
    fi
    
    log_info "开始安装插件和依赖..."
    
    # 创建目录
    local lib_dir="$SEATUNNEL_HOME/lib"
    local connectors_dir="$SEATUNNEL_HOME/connectors"
    create_directory "$lib_dir"
    create_directory "$connectors_dir"
    setup_permissions "$lib_dir"
    setup_permissions "$connectors_dir"
    
    # 如果CONNECTORS为空，使用默认值
    if [ -z "$CONNECTORS" ]; then
        CONNECTORS="jdbc,hive"
        log_info "使用默认连接器: $CONNECTORS"
    fi
    
    # 读取启用的连接器列表
    IFS=',' read -r -a enabled_connectors <<< "${CONNECTORS}"
    
    # 使用全局变量PLUGIN_REPO，已在read_config中设置
    local retries=3
    local config_file="$EXEC_PATH/config.properties"  # 添加这行
    
    # 处理每个连接器
    for connector in "${enabled_connectors[@]}"; do
        connector=$(echo "$connector" | tr -d '[:space:]')
        log_info "处理连接器: $connector"
        
        # 下载连接器插件
        local plugin_jar="connector-${connector}-${SEATUNNEL_VERSION}.jar"
        local target_path="$connectors_dir/$plugin_jar"
        
        # 检查插件是否已存在
        if [ -f "$target_path" ]; then
            log_info "连接器插件已存在: $plugin_jar"
        else
            log_info "下载连接器插件: $plugin_jar"
            download_artifact "$target_path" "$connector" "plugin"
        fi
        
        # 读取并处理连接器的依赖库
        local libs_str
        libs_str=$(grep "^${connector}_libs=" "$config_file" | cut -d'=' -f2 || true)
                  
        if [ -n "$libs_str" ]; then
            log_info "处理 $connector 连接器的依赖库..."
            IFS=',' read -r -a libs <<< "$libs_str"
            for lib in "${libs[@]}"; do
                lib=$(echo "$lib" | tr -d '[:space:]')  # 移除空白字符
                IFS=':' read -r group_id artifact_id version <<< "$lib"
                local lib_name="${artifact_id}-${version}.jar"
                local lib_path="$lib_dir/$lib_name"
                
                # 检查依赖库是否已存在
                if [ -f "$lib_path" ]; then
                    log_info "依赖库已存在: $lib_name"
                else
                    log_info "下载依赖库: $lib_name"
                    download_artifact "$lib_path" "$lib" "lib"
                fi
            done
        else
            log_info "连接器 $connector 没有配置依赖库"
        fi
    done
    
    log_info "插件和依赖安装完成"
}

# 下载构件（插件或库）
download_artifact() {
    local target_path=$1
    local artifact=$2
    local type=$3
    local retry_count=0
    local download_success=false
    local max_retries=3
    local retry_delay=2
    
    while [ $retry_count -lt $max_retries ]; do
        log_info "下载尝试 $((retry_count + 1))/$max_retries"
        
        # 构建下载URL
        local download_url
        local repo_url="${PLUGIN_REPOS[$PLUGIN_REPO]:-${PLUGIN_REPOS[aliyun]}}"  # 使用aliyun作为默认值
        
        if [ "$type" = "plugin" ]; then
            # 标准仓库的插件URL格式
            download_url="$repo_url/org/apache/seatunnel/connector-${artifact}/${SEATUNNEL_VERSION}/connector-${artifact}-${SEATUNNEL_VERSION}.jar"
        else
            # 处理依赖库的URL
            IFS=':' read -r group_id artifact_id version <<< "$artifact"
            group_path=$(echo "$group_id" | tr '.' '/')
            download_url="$repo_url/$group_path/$artifact_id/$version/$artifact_id-$version.jar"
        fi
        
        log_info "从 $download_url 下载..."
        
        # 检查URL是否可访问
        if ! check_url "$download_url"; then
            log_warning "当前下载源不可用，尝试切换到备用源..."
            if [ "$PLUGIN_REPO" = "aliyun" ]; then
                PLUGIN_REPO="apache"
                continue
            elif [ "$PLUGIN_REPO" = "huaweicloud" ]; then
                PLUGIN_REPO="aliyun"
                continue
            fi
        fi
        
        # 使用curl下载
        if curl -L \
            --fail \
            --progress-bar \
            --connect-timeout 20 \
            --retry 3 \
            --retry-delay 2 \
            --retry-max-time 60 \
            -o "$target_path" \
            "$download_url" 2>&1; then
            
            if [ -f "$target_path" ]; then
                chmod 644 "$target_path"
                chown "$INSTALL_USER:$INSTALL_GROUP" "$target_path"
                log_info "下载成功: $(basename "$target_path")"
                download_success=true
                break
            fi
        fi
        
        retry_count=$((retry_count + 1))
        if [ $retry_count -lt $max_retries ]; then
            log_warning "下载失败，等待 ${retry_delay} 秒后重试..."
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))  # 指数退避
        fi
    done
    
    if [ "$download_success" != "true" ]; then
        log_error "下载失败: $download_url，已重试 $max_retries 次"
        return 1
    fi
}

# 修改create_service_file函数，添加必要的参数
create_service_file() {
    local service_name=$1
    local role=$2
    # 使用统一的 JAVA_HOME 路径，确保所有节点一致
    local java_home=${3:-"$(get_unified_java_home)"}
    local seatunnel_home=${4:-"$SEATUNNEL_HOME"}
    local install_user=${5:-"$INSTALL_USER"}
    local install_group=${6:-"$INSTALL_GROUP"}
    
    local service_file="/etc/systemd/system/${service_name}.service"
    local description="Apache SeaTunnel ${role} Service"
    local exec_args=""
    local hazelcast_config=""
    local jvm_options_file=""
    
    # 设置配置文件和参数
    case "$role" in
        "Hybrid")
            exec_args=""
            seatunnel_logs="seatunnel-engine-server"
            hazelcast_config="${seatunnel_home}/config/hazelcast.yaml"
            jvm_options_file="${seatunnel_home}/config/jvm_options"
            ;;
        "Master")
            exec_args="-r master"
            seatunnel_logs="seatunnel-engine-master"
            hazelcast_config="${seatunnel_home}/config/hazelcast-master.yaml"
            jvm_options_file="${seatunnel_home}/config/jvm_master_options"
            ;;
        "Worker")
            exec_args="-r worker"
            seatunnel_logs="seatunnel-engine-worker"
            hazelcast_config="${seatunnel_home}/config/hazelcast-worker.yaml"
            jvm_options_file="${seatunnel_home}/config/jvm_worker_options"
            ;;
    esac

    # 从JVM配置文件读取堆内存大小
    # SeaTunnel 2.3.9+ 的 jvm_options 文件中 -Xmx 可能是注释状态: # -Xmx2g
    # 需要兼容两种格式: -Xmx2g 或 # -Xmx2g
    local heap_size
    heap_size=$(grep -E '^#?[[:space:]]*-Xmx[0-9]+g' "$jvm_options_file" | sed 's/^#[[:space:]]*//' | sed 's/-Xmx\([0-9]\+\)g/\1/' || echo "2")
    
    # 读取所有JVM配置
    local jvm_opts=""
    while IFS= read -r line; do
        [[ "$line" =~ ^[[:space:]]*# ]] && continue
        [[ -z "${line// }" ]] && continue
        if [[ "$line" =~ ^-XX ]]; then
            if [[ "$line" =~ HeapDumpPath ]]; then
                jvm_opts+="${line/\/tmp\/seatunnel\/dump\/zeta-server/${seatunnel_home}\/dump\/seatunnel-zeta-server} "
            else
                jvm_opts+="$line "
            fi
        fi
    done < "$jvm_options_file"

    # 创建临时服务文件
    local temp_service_file="/tmp/${service_name}.service.tmp"
    cat > "$temp_service_file" << EOF
[Unit]
Description=${description}
After=network.target

[Service]
Type=simple
User=${install_user}
Group=${install_group}
Environment="JAVA_HOME=${java_home}"
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:\${JAVA_HOME}/bin"
Environment="SEATUNNEL_HOME=${seatunnel_home}"
WorkingDirectory=${seatunnel_home}
ExecStart=${java_home}/bin/java \\
    -Dlog4j2.contextSelector=org.apache.logging.log4j.core.async.AsyncLoggerContextSelector \\
    -Dhazelcast.logging.type=log4j2 \\
    -Dlog4j2.configurationFile=${seatunnel_home}/config/log4j2.properties \\
    -Dseatunnel.logs.path=${seatunnel_home}/logs \\
    -Dseatunnel.logs.file_name=${seatunnel_logs} \\
    -Xms${heap_size}g \\
    -Xmx${heap_size}g \\
    ${jvm_opts}\\
    -Dseatunnel.config=${seatunnel_home}/config/seatunnel.yaml \\
    -Dhazelcast.config=${hazelcast_config} \\
    -cp "${seatunnel_home}/lib/*:${seatunnel_home}/starter/seatunnel-starter.jar" \\
    org.apache.seatunnel.core.starter.seatunnel.SeaTunnelServer ${exec_args}
ExecStop=/bin/kill -s TERM \$MAINPID
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    # 使用sudo移动服务文件到系统目录
    sudo mv "$temp_service_file" "$service_file"
    
    # 设置权限和所有者
    sudo chown root:root "$service_file"
    sudo chmod 644 "$service_file"
    
    # 重新加载systemd配置
    sudo systemctl daemon-reload
    
    # 启用服务
    sudo systemctl enable "$(basename "$service_file")"
}

# 修改setup_auto_start函数
setup_auto_start() {
    if [ "${ENABLE_AUTO_START}" != "true" ]; then
        log_info "跳过开机自启动配置"
        return 0
    fi
    
    log_info "配置SeaTunnel开机自启动..."
    
    local current_ip
    current_ip=$(hostname -I | awk '{print $1}')
    
    setup_remote_service() {
        local node=$1
        local service_name=$2
        local role=$3
        
        log_info "在节点 $node 上配置服务..."
        
        # 在本地生成服务文件
        create_service_file "$service_name" "$role"
        
        # 复制服务文件到远程节点
        scp_with_retry "/etc/systemd/system/${service_name}.service" "$node" "/tmp/${service_name}.service"
        
        # 在远程节点上安装服务文件
        ssh_with_retry "$node" "sudo mv /tmp/${service_name}.service /etc/systemd/system/ && \
            sudo chown root:root /etc/systemd/system/${service_name}.service && \
            sudo chmod 644 /etc/systemd/system/${service_name}.service && \
            sudo systemctl daemon-reload && \
            sudo systemctl enable ${service_name}"
    }
    
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式
        for node in "${ALL_NODES[@]}"; do
            if [ "$node" = "$current_ip" ] || [ "$node" = "localhost" ]; then
                log_info "在本地节点配置服务..."
                create_service_file "seatunnel" "Hybrid"
            else
                setup_remote_service "$node" "seatunnel" "Hybrid"
            fi
        done
    else
        # 分离模式
        # 配置Master节点
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" = "$current_ip" ] || [ "$master" = "localhost" ]; then
                log_info "在本地Master节点配置服务..."
                create_service_file "seatunnel-master" "Master"
            else
                setup_remote_service "$master" "seatunnel-master" "Master"
            fi
        done
        
        # 配置Worker节点
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" = "$current_ip" ] || [ "$worker" = "localhost" ]; then
                log_info "在本地Worker节点配置服务..."
                create_service_file "seatunnel-worker" "Worker"
            else
                setup_remote_service "$worker" "seatunnel-worker" "Worker"
            fi
        done
    fi
    
    log_success "服务配置完成"
}

# 检查系统内存
check_memory() {
    log_info "检查系统内存..."
    
    # 获取系统总内存(GB)
    local total_mem
    if [ -f /proc/meminfo ]; then
        total_mem=$(awk '/MemTotal/ {print int($2/1024/1024)}' /proc/meminfo)
    else
        # 对于不支持 /proc/meminfo 的系统，尝试使用其他命令
        if command -v free >/dev/null 2>&1; then
            total_mem=$(free -g | awk '/Mem:/ {print int($2)}')
        else
            log_error "无法获取系统内存信息"
        fi
    fi
    
    # 获取可用内存(GB)
    local available_mem
    if [ -f /proc/meminfo ]; then
        available_mem=$(awk '/MemAvailable/ {print int($2/1024/1024)}' /proc/meminfo)
    else
        if command -v free >/dev/null 2>&1; then
            available_mem=$(free -g | awk '/Mem:/ {print int($4)}')
        else
            log_error "无法获取系统可用内存信息"
        fi
    fi
    
    # 根据部署模式检查内存需求
    local required_mem=0
    local current_ip
    current_ip=$(hostname -I | awk '{print $1}')
    
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        required_mem=$((HYBRID_HEAP_SIZE + 2)) # 额外预留2GB系统使用
        log_info "混合模式下需要 ${HYBRID_HEAP_SIZE}GB 堆内存 + 2GB 系统预留"
    else
        local is_master=false
        local is_worker=false
        
        # 检查当前机器是否为Master节点
        if [[ " ${MASTER_IPS[*]} " =~ " $current_ip " ]]; then
            is_master=true
            required_mem=$((MASTER_HEAP_SIZE))
            log_info "当前节点是Master节点，需要 ${MASTER_HEAP_SIZE}GB 堆内存"
        fi
        
        # 检查当前机器是否为Worker节点
        if [[ " ${WORKER_IPS[*]} " =~ " $current_ip " ]]; then
            is_worker=true
            if [ "$is_master" = true ]; then
                # 如果同时是Master和Worker，需要两者内存之和
                required_mem=$((required_mem + WORKER_HEAP_SIZE))
                log_info "当前节点同时是Worker节点，额外需要 ${WORKER_HEAP_SIZE}GB 堆内存"
                log_info "总共需 ${required_mem}GB 堆内存 + 2GB 系统预留"
            else
                required_mem=$((WORKER_HEAP_SIZE))
                log_info "当前节点是Worker节点，需要 ${WORKER_HEAP_SIZE}GB 堆内存"
            fi
        fi
        
        # 添加系统预留
        required_mem=$((required_mem + 2))
    fi
    
    log_info "系统总内存: ${total_mem}GB"
    log_info "系统可用内存: ${available_mem}GB"
    log_info "最小所需内存: ${required_mem}GB"
    
    # 检查总内存是否足够
    if [ $total_mem -lt $required_mem ]; then
        log_error "系统总内存不足！需要至少 ${required_mem}GB，当前只有 ${total_mem}GB"
    fi
    
    # 检查可用内存是否足够
    if [ $available_mem -lt $required_mem ]; then
        log_warning "系统可用内存不足！需要至少 ${required_mem}GB，当前只有 ${available_mem}GB"
        log_warning "建议释放一些内存后再继续安装"
        read -r -p "是否继续安装? [y/N] " response
        case "$response" in
            [yY][eE][sS]|[yY]) 
                log_warning "继续安装，但可能会影响系统性能"
                ;;
            *)
                log_error "安装已取消"
                ;;
        esac
    fi
}

# 处理安装包
handle_package() {
    if [ "$INSTALL_MODE" = "online" ]; then
        # 在线安装模式
        local package_name="apache-seatunnel-${SEATUNNEL_VERSION}-bin.tar.gz"
        local package_path
        
        # 使用全局下载目录检查文件
        if [ -f "$DOWNLOAD_DIR/$package_name" ]; then
            log_warning "发现本地已存在安装包: $DOWNLOAD_DIR/$package_name"
            read -r -p "是否重新下载? [y/N] " response
            case "$response" in
                [yY][eE][sS]|[yY])
                    rm -f "$DOWNLOAD_DIR/$package_name"
                    ;;
                *)
                    log_info "使用已存在的安装包"
                    package_path="$DOWNLOAD_DIR/$package_name"
                    ;;
            esac
        fi
        
        # 下载安装包
        if [ -z "$package_path" ]; then
            if download_package "$package_name" "$SEATUNNEL_VERSION"; then
                package_path=$(cat /tmp/download_path.tmp)
                rm -f /tmp/download_path.tmp
                
                if [ ! -f "$package_path" ]; then
                    log_error "下载的安装包不存在: $package_path"
                fi
            else
                log_error "安装包下载失败"
            fi
        fi
        
        PACKAGE_PATH="$package_path"
    fi
    
    # 验证安装包
    verify_package "$PACKAGE_PATH"
    
    # 创建安装目录
    log_info "创建安装目录..."
    sudo mkdir -p "$BASE_DIR"
    sudo chown "$INSTALL_USER:$INSTALL_GROUP" "$BASE_DIR"
    
    # 解压安装包
    log_info "解压安装包..."
    cd "$BASE_DIR" || log_error "无法进入安装目录"
    sudo tar -zxf "$PACKAGE_PATH"
    
    # 立即修改解压后目录的所有权
    log_info "设置目录权限..."
    sudo chown -R "$INSTALL_USER:$INSTALL_GROUP" "$SEATUNNEL_HOME"
    sudo chmod -R 755 "$SEATUNNEL_HOME"
    
    # 验证权限设置
    if [ ! -w "$SEATUNNEL_HOME" ]; then
        log_error "无法写入安装目录: $SEATUNNEL_HOME，请检查权限设置"
    fi
    
    log_success "安装包处理完成"
}

# 分发到其他节点
distribute_to_nodes() {
    log_info "分发到其他节点..."
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式：分发到所有节点
        for node in "${ALL_NODES[@]}"; do
            # 跳过localhost和当前节点
            if [ "$node" = "localhost" ] || [ "$node" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地节点: $node"
                continue
            fi
            
            log_info "分发到 $node..."
            ssh_with_retry "$node" "mkdir -p $BASE_DIR && chown $INSTALL_USER:$INSTALL_GROUP $BASE_DIR"
            scp_with_retry "$SEATUNNEL_HOME" "$node" "$BASE_DIR/"
        done
    else
        # 分离模式：分发到master和worker节点
        # 先分发到其他master节点
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" = "localhost" ] || [ "$master" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地Master节点: $master"
                continue
            fi
            
            log_info "分发到Master节点 $master..."
            ssh_with_retry "$master" "mkdir -p $BASE_DIR && chown $INSTALL_USER:$INSTALL_GROUP $BASE_DIR"
            scp_with_retry "$SEATUNNEL_HOME" "$master" "$BASE_DIR/"
        done
        
        # 再分发到worker节点
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" = "localhost" ] || [ "$worker" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地Worker节点: $worker"
                continue
            fi
            
            log_info "分发到Worker节点 $worker..."
            ssh_with_retry "$worker" "mkdir -p $BASE_DIR && chown $INSTALL_USER:$INSTALL_GROUP $BASE_DIR"
            scp_with_retry "$SEATUNNEL_HOME" "$worker" "$BASE_DIR/"
        done
    fi
}

# 配置环境变量
setup_environment() {
    log_info "配置环境变量..."
    BASHRC_CONTENT="
# SEATUNNEL_HOME BEGIN
export SEATUNNEL_HOME=$SEATUNNEL_HOME
export PATH=\$PATH:\$SEATUNNEL_HOME/bin
# SEATUNNEL_HOME END"

    # 获取用户home目录
    USER_HOME=""
    if command -v getent >/dev/null 2>&1; then
        USER_HOME=$(getent passwd "$INSTALL_USER" | cut -d: -f6)
    else
        USER_HOME=$(eval echo ~"$INSTALL_USER")
    fi
    
    if [ -z "$USER_HOME" ]; then
        log_error "无法获取用户 $INSTALL_USER 的home目录"
    fi
    
    # 配置本地环境变量
    if grep -q "SEATUNNEL_HOME" "$USER_HOME/.bashrc"; then
        log_info "本地环境变量已存在，更新配置..."
        sed -i '/# SEATUNNEL_HOME BEGIN/,/# SEATUNNEL_HOME END/d' "$USER_HOME/.bashrc"
    fi
    echo "$BASHRC_CONTENT" >> "$USER_HOME/.bashrc"
    
    # 远程节点环境变量
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式：配置所有远程节点
        for node in "${ALL_NODES[@]}"; do
            # 跳过localhost和当前节点
            if [ "$node" = "localhost" ] || [ "$node" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地节点环境变量配置: $node"
                continue
            fi
            
            log_info "配置节点 $node 的环境变量..."
            remote_home=$(ssh_with_retry "$node" "echo ~$INSTALL_USER")
            ssh_with_retry "$node" "
                if grep -q 'SEATUNNEL_HOME' '$remote_home/.bashrc'; then
                    sed -i '/# SEATUNNEL_HOME BEGIN/,/# SEATUNNEL_HOME END/d' '$remote_home/.bashrc'
                fi
                echo '$BASHRC_CONTENT' >> '$remote_home/.bashrc'
            "
        done
    else
        # 分离模式：配置master和worker节点
        # 配置其他master节点
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" = "localhost" ] || [ "$master" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地Master节点环境变量配置: $master"
                continue
            fi
            
            log_info "配置Master节点 $master 的环境变量..."
            remote_home=$(ssh_with_retry "$master" "echo ~$INSTALL_USER")
            ssh_with_retry "$master" "
                if grep -q 'SEATUNNEL_HOME' '$remote_home/.bashrc'; then
                    sed -i '/# SEATUNNEL_HOME BEGIN/,/# SEATUNNEL_HOME END/d' '$remote_home/.bashrc'
                fi
                echo '$BASHRC_CONTENT' >> '$remote_home/.bashrc'
            "
        done
        
        # 配置worker节点
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" = "localhost" ] || [ "$worker" = "$(hostname -I | awk '{print $1}')" ]; then
                log_info "跳过本地Worker节点环境变量配置: $worker"
                continue
            fi
            
            log_info "配置Worker节点 $worker 的环境变量..."
            remote_home=$(ssh_with_retry "$worker" "echo ~$INSTALL_USER")
            ssh_with_retry "$worker" "
                if grep -q 'SEATUNNEL_HOME' '$remote_home/.bashrc'; then
                    sed -i '/# SEATUNNEL_HOME BEGIN/,/# SEATUNNEL_HOME END/d' '$remote_home/.bashrc'
                fi
                echo '$BASHRC_CONTENT' >> '$remote_home/.bashrc'
            "
        done
    fi
}

function show_completion_info(){
    # 计算安装时长
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    MINUTES=$((DURATION / 60))
    SECONDS=$((DURATION % 60))
    
    # 添加安装完成后的验证提示
    echo -e "\n${GREEN}SeaTunnel安装完成!${NC}"
    echo -e "安装总耗时: ${GREEN}${MINUTES}分${SECONDS}秒${NC}"
    echo -e "\n${YELLOW}验证和使用说明:${NC}"
    echo "1. 刷新环境变量:"
    echo -e "${GREEN}source $USER_HOME/.bashrc${NC}"
    
    echo -e "\n2. 集群管理命令:"
    if [ "${ENABLE_AUTO_START}" = "true" ]; then
        if [ "$DEPLOY_MODE" = "hybrid" ]; then
            echo -e "启动服务:    ${GREEN}sudo systemctl start seatunnel${NC}"
            echo -e "停止服务:    ${GREEN}sudo systemctl stop seatunnel${NC}"
            echo -e "重启服务:    ${GREEN}sudo systemctl restart seatunnel${NC}"
            echo -e "查看状态:    ${GREEN}sudo systemctl status seatunnel${NC}"
            echo -e "查看启动日志:    ${GREEN}sudo journalctl -u seatunnel -n 100 --no-pager${NC}"
            echo -e "查看运行日志:    ${GREEN}tail -n 100 $SEATUNNEL_HOME/logs/seatunnel-engine-server.out${NC}"
        else
            echo -e "Master服务命令:"
            echo -e "启动服务:    ${GREEN}sudo systemctl start seatunnel-master${NC}"
            echo -e "停止服务:    ${GREEN}sudo systemctl stop seatunnel-master${NC}"
            echo -e "重启服务:    ${GREEN}sudo systemctl restart seatunnel-master${NC}"
            echo -e "查看状态:    ${GREEN}sudo systemctl status seatunnel-master${NC}"
            echo -e "查看启动日志:    ${GREEN}sudo journalctl -u seatunnel-master -n 100 --no-pager${NC}"
            echo -e "查看运行日志:    ${GREEN}tail -n 100 $SEATUNNEL_HOME/logs/seatunnel-engine-master.log${NC}"
            echo -e "----------------------------------------"
            echo -e "\nWorker服务命令:"
            echo -e "启动服务:    ${GREEN}sudo systemctl start seatunnel-worker${NC}"
            echo -e "停止服务:    ${GREEN}sudo systemctl stop seatunnel-worker${NC}"
            echo -e "重启服务:    ${GREEN}sudo systemctl restart seatunnel-worker${NC}"
            echo -e "查看状态:    ${GREEN}sudo systemctl status seatunnel-worker${NC}"
            echo -e "查看启动日志:    ${GREEN}sudo journalctl -u seatunnel-worker -n 100 --no-pager${NC}"
            echo -e "查看运行日志:    ${GREEN}tail -n 100 $SEATUNNEL_HOME/logs/seatunnel-engine-worker.log${NC}"
        fi
    else
        echo -e "启动集群:    ${GREEN}$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh start${NC}"
        echo -e "停止集群:    ${GREEN}$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh stop${NC}"
        echo -e "重启集群:    ${GREEN}$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh restart${NC}"
        echo -e "查看运行日志:    ${GREEN}tail -n 100 $SEATUNNEL_HOME/logs/seatunnel-engine-server.out${NC}"
    fi
    
    echo -e "\n3. 验证安装:"
    echo -e "运行示例任务: ${GREEN}$SEATUNNEL_HOME/bin/seatunnel.sh --config config/v2.batch.config.template${NC}"
    
    echo -e "\n${YELLOW}部署信息:${NC}"
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        echo "部署模式: 混合模式"
        echo -e "集群节点: ${GREEN}${CLUSTER_NODES[*]}${NC}"
    else
        echo "部署模式: 分离模式"
        echo -e "Master节点: ${GREEN}${MASTER_IPS[*]}${NC}"
        echo -e "Worker节点: ${GREEN}${WORKER_IPS[*]}${NC}"
    fi
    
    echo -e "\n${YELLOW}注意事项:${NC}"
    echo "1. 首次启动集群前，请确保所有节点的环境变量已经生效,source $USER_HOME/.bashrc"
    echo -e "2. 更多使用说明请参考：${GREEN}https://seatunnel.apache.org/docs${NC}"
}


# 在颜色定义后添加
# 错误追踪函数
trace_error() {
    local err=$?
    local line_no=$1
    local bash_command=$2
    
    # 避免递归错误
    if [ "${IN_ERROR_HANDLER:-0}" -eq 1 ]; then
        echo "致命错误: 在错误处理过程中发生错误"
        exit 1
    fi
    export IN_ERROR_HANDLER=1
    
    echo -e "\n${RED}[ERROR TRACE]${NC} $(date '+%Y-%m-%d %H:%M:%S')"
    echo -e "${RED}错误码:${NC} $err"
    echo -e "${RED}错误行号:${NC} $line_no"
    echo -e "${RED}错误命令:${NC} $bash_command"
    
    # 输出函数调用栈
    local frame=0
    echo -e "${RED}函数调用栈:${NC}"
    while caller $frame; do
        ((frame++))
    done | awk '{printf "  %s(): 第%s行 in %s\n", $2, $1, $3}'
    
    # 如果是在函数中发生错误,显示函数名
    if [[ "${FUNCNAME[*]}" ]]; then
        echo -e "${RED}当前函数:${NC} ${FUNCNAME[1]}"
    fi
    
    # 显示最后几行日志
    echo -e "${RED}最后10行日志:${NC}"
    tail -n 10 "$LOG_DIR/install.log" 2>/dev/null
    
    # 清理并退出
    cleanup
    exit $err
}

# 设置错误追踪
set -E           # 继承ERR trap
set -o pipefail  # 管道中的错误也会被捕获
trap 'trace_error ${LINENO} "$BASH_COMMAND"' ERR

# 创建日志目录
mkdir -p "$LOG_DIR"
chmod 755 "$LOG_DIR"
[ -n "$INSTALL_USER" ] && [ -n "$INSTALL_GROUP" ] && chown "$INSTALL_USER:$INSTALL_GROUP" "$LOG_DIR"

# 创建日志文件
touch "$LOG_FILE"
chmod 644 "$LOG_FILE"

# 如果INSTALL_USER已定义,设置目录和文件所有权
if [ -n "$INSTALL_USER" ] && [ -n "$INSTALL_GROUP" ]; then
    chown "$INSTALL_USER:$INSTALL_GROUP" "$LOG_DIR"
    chown "$INSTALL_USER:$INSTALL_GROUP" "$LOG_FILE"
fi

# 重定向输出到日志文件
exec 1> >(tee -a "$LOG_FILE")
exec 2>&1

# 记录脚本开始执行
log_info "开始执行安装脚本..."
log_info "脚本路径: $0"
log_info "脚本参数: $*"

# 添加处理SELinux的函数
handle_selinux() {
    log_info "检查SELinux状态..."
    
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        # 混合模式：检查所有节点
        for node in "${ALL_NODES[@]}"; do
            if [ "$node" != "localhost" ] && [ "$node" != "$(hostname -I | awk '{print $1}')" ]; then
                check_and_handle_selinux "$node" true
            else 
                check_and_handle_selinux "localhost" false
            fi
        done
    else
        # 分离模式：检查master和worker节点
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" != "localhost" ] && [ "$master" != "$(hostname -I | awk '{print $1}')" ]; then
                check_and_handle_selinux "$master" true
            else 
                check_and_handle_selinux "localhost" false
            fi
        done
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" != "localhost" ] && [ "$worker" != "$(hostname -I | awk '{print $1}')" ]; then
                check_and_handle_selinux "$worker" true
            else 
                check_and_handle_selinux "localhost" false
            fi
        done
    fi
}

# 将check_and_handle_selinux提取为独立函数
check_and_handle_selinux() {
    local node=$1
    local is_remote=$2
    
    local check_script='
    check_and_disable_selinux() {
        if ! command -v sestatus >/dev/null 2>&1; then
            echo "NO_SELINUX"
            exit 0
        fi
    
        # 检查SELinux状态
        if sestatus | grep -i "disabled" >/dev/null 2>&1; then
            echo "SELINUX_DISABLED"
            exit 0
        fi
        
        # 检查是否为enforcing模式
        if sestatus | grep -i "enforcing" >/dev/null 2>&1; then
            # 先设置为permissive模式
            sudo setenforce 0
            
            # 修改配置文件以永久禁用
            if [ -f "/etc/selinux/config" ]; then
                sudo sed -i "s/^[[:space:]]*SELINUX=enforcing/SELINUX=disabled/" /etc/selinux/config
                sudo sed -i "s/^[[:space:]]*SELINUX=permissive/SELINUX=disabled/" /etc/selinux/config
                echo "SELINUX_WILL_DISABLE"
            else
                echo "CONFIG_NOT_FOUND"
            fi
        else
            # 如果是permissive模式,也永久禁用
            if [ -f "/etc/selinux/config" ]; then
                sudo sed -i "s/^[[:space:]]*SELINUX=permissive/SELINUX=disabled/" /etc/selinux/config
                echo "SELINUX_WILL_DISABLE"
            else
                echo "CONFIG_NOT_FOUND"
            fi
        fi
    }

    check_and_disable_selinux
    '
    
    if [ "$is_remote" = true ]; then
        # 远程节点检查
        local result
        result=$(execute_remote_script "$node" "$check_script")
        
        case "$result" in
            "NO_SELINUX")
                log_info "节点 $node 未安装SELinux,无需处理"
                ;;
            "SELINUX_DISABLED")
                log_info "节点 $node 的SELinux已禁用"
                ;;
            "SELINUX_WILL_DISABLE")
                log_warning "节点 $node 的SELinux已设置为禁用状态,重启后生效"
                ;;
            "CONFIG_NOT_FOUND")
                log_warning "节点 $node 未找到SELinux配置文件,请手动检查"
                ;;
            *)
                log_warning "节点 $node 的SELinux状态未知: $result"
                ;;
        esac
    else
        # 本地节点检查
        if ! command -v sestatus >/dev/null 2>&1; then
            log_info "本地节点未安装SELinux,无需处理"
            return 0
        fi
        
        if sestatus | grep -i "disabled" >/dev/null 2>&1; then
            log_info "本地节点SELinux已禁用"
            return 0
        fi
        
        # 检查是否为enforcing模式
        if sestatus | grep -i "enforcing" >/dev/null 2>&1; then
            log_warning "本地节点SELinux处于enforcing模式,正在禁用..."
            # 先设置为permissive模式
            sudo setenforce 0
            
            # 修改配置文件以永久禁用
            if [ -f "/etc/selinux/config" ]; then
                sudo sed -i "s/^[[:space:]]*SELINUX=enforcing/SELINUX=disabled/" /etc/selinux/config
                sudo sed -i "s/^[[:space:]]*SELINUX=permissive/SELINUX=disabled/" /etc/selinux/config
                log_warning "本地节点SELinux已设置为禁用状态,重启后生效"
            else
                log_warning "本地节点未找到SELinux配置文件,请手动检查"
            fi
        else
            # 如果是permissive模式,也永久禁用
            if [ -f "/etc/selinux/config" ]; then
                sudo sed -i "s/^[[:space:]]*SELINUX=permissive/SELINUX=disabled/" /etc/selinux/config
                log_warning "本地节点SELinux已设置为禁用状态,重启后生效"
            else
                log_warning "本地节点未找到SELinux配置文件,请手动检查"
            fi
        fi
    fi
    
    # 显示SELinux禁用警告
    log_warning "
注意: 为了确保SeaTunnel能够正常运行,安装脚本已禁用SELinux。
这可能会降低系统的安全性,但可以避免权限问题导致的各种错误。
如果您需要重新启用SELinux,请在安装完成后修改/etc/selinux/config文件,
并执行'sudo setenforce 1'命令(重启后生效)。

更多信息请参考README文档中的SELinux相关说明。"
}

# 添加execute_remote_script函数
execute_remote_script() {
    local node=$1
    local script=$2
    shift 2  # 移除前两个参数，剩余的都是要传递给脚本的参数
    local TIMEOUT=20
    
    # 创建临时脚本文件，添加参数处理
    local temp_script="/tmp/remote_script_$RANDOM.sh"
    {
        echo '#!/bin/bash'
        # 将参数数组重建为脚本变量
        echo 'SCRIPT_ARGS=('
        printf "'%s' " "$@"
        echo ')'
        echo "$script"
    } > "$temp_script"
    chmod +x "$temp_script"
    
    # 复制脚本到远程节点并执行，添加超时控制
    if ! timeout $TIMEOUT scp -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$temp_script" "${INSTALL_USER}@${node}:/tmp/" >/dev/null 2>&1; then
        log_error "向节点 $node 传输脚本失败"
        rm -f "$temp_script"
        return 1
    fi
    
    local result
    result=$(timeout $TIMEOUT ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "bash /tmp/$(basename "$temp_script")")
    local exit_code=$?
    
    # 清理临时文件
    rm -f "$temp_script"
    timeout $TIMEOUT ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "${INSTALL_USER}@${node}" "rm -f /tmp/$(basename "$temp_script")" >/dev/null 2>&1
    
    # 返回结果
    echo "$result"
    return $exit_code
}

# 检查防火墙状态
check_firewall() {
    # 转换为小写进行比较
    local check_firewall_lower=$(echo "$CHECK_FIREWALL" | tr '[:upper:]' '[:lower:]')
    if [ "$check_firewall_lower" = "false" ]; then
        log_info "已禁用防火墙检查"
        return 0
    fi
    
    log_info "检查防火墙状态..."
    
    check_node_firewall() {
        local node=$1
        local is_remote=$2
        local ports=()
        local firewall_status_shown=false
        local ports_shown=false
        
        # 根据部署模式确定需要检查的端口
        if [ "$DEPLOY_MODE" = "hybrid" ]; then
            ports+=("${HYBRID_PORT:-5801}")
            if is_seatunnel_ge_239; then
                ports+=("${MASTER_HTTP_PORT:-8080}")
            fi
        else
            if [[ " ${MASTER_IPS[*]} " =~ " $node " ]]; then
                ports+=("${MASTER_PORT:-5801}")
                # 添加Master HTTP API端口检查 (SeaTunnel 2.3.9+)
                if is_seatunnel_ge_239; then
                    ports+=("${MASTER_HTTP_PORT:-8080}")
                fi
            fi
            if [[ " ${WORKER_IPS[*]} " =~ " $node " ]]; then
                ports+=("${WORKER_PORT:-5802}")
            fi
        fi
        
        # 添加SSH端口
        ports+=("$SSH_PORT")
        
        # 优化端口显示,只在第一次显示
        if [ ! "$ports_shown" ]; then
            log_info "检查节点 $node 的端口: ${ports[*]}/tcp"
            ports_shown=true
        fi

        local check_script='
            # 获取传入的端口参数
            ports=("${SCRIPT_ARGS[@]}")
            
            # 检查防火墙状态和端口
            if command -v systemctl >/dev/null 2>&1; then
                if sudo systemctl is-active --quiet firewalld; then
                    # 检查端口是否开放
                    ports_status=""
                    for port in "${ports[@]}"; do
                        if sudo firewall-cmd --list-ports | grep -w "$port/tcp" >/dev/null 2>&1; then
                            ports_status="${ports_status}${port}:open,"
                        else
                            ports_status="${ports_status}${port}:closed,"
                        fi
                    done
                    echo "FIREWALLD_ACTIVE:$ports_status"
                elif sudo systemctl is-active --quiet ufw; then
                    # 检查UFW端口状态
                    ports_status=""
                    for port in "${ports[@]}"; do
                        if sudo ufw status | grep -w "$port/tcp.*ALLOW" >/dev/null 2>&1; then
                            ports_status="${ports_status}${port}:open,"
                        else
                            ports_status="${ports_status}${port}:closed,"
                        fi
                    done
                    echo "UFW_ACTIVE:$ports_status"
                else
                    echo "FIREWALL_INACTIVE"
                fi
            elif command -v service >/dev/null 2>&1; then
                if sudo service iptables status >/dev/null 2>&1; then
                    # 检查iptables端口状态
                    ports_status=""
                    for port in "${ports[@]}"; do
                        if sudo iptables -L -n | grep -w "tcp dpt:$port.*ACCEPT" >/dev/null 2>&1; then
                            ports_status="${ports_status}${port}:open,"
                        else
                            ports_status="${ports_status}${port}:closed,"
                        fi
                    done
                    echo "IPTABLES_ACTIVE:$ports_status"
                else
                    echo "FIREWALL_INACTIVE"
                fi
            else
                echo "NO_FIREWALL"
            fi'
        
        if [ "$is_remote" = true ]; then
            # 远程节点检查，传递端口参数
            local result
            result=$(execute_remote_script "$node" "$check_script" "${ports[@]}")
            
            case "${result%%:*}" in
                "FIREWALLD_ACTIVE"|"UFW_ACTIVE"|"IPTABLES_ACTIVE")
                    local firewall_type="${result%%:*}"
                    local ports_status="${result#*:}"
                    local closed_ports=()
                    
                    # 解析端口状态
                    IFS=',' read -r -a port_array <<< "$ports_status"
                    for port_status in "${port_array[@]}"; do
                        if [[ "$port_status" =~ ([0-9]+):closed ]]; then
                            closed_ports+=("${BASH_REMATCH[1]}")
                        fi
                    done
                    
                    if [ ${#closed_ports[@]} -gt 0 ]; then
                        log_warning "节点 $node 的防火墙已启用，以下端口未开放: ${closed_ports[*]}"
                        echo -e "\n${YELLOW}请在节点 $node 上执行以下命令开放端口:${NC}"
                        case "$firewall_type" in
                            "FIREWALLD_ACTIVE")
                                echo -e "1. 使用root用户执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}firewall-cmd --permanent --add-port=$port/tcp${NC}"
                                done
                                echo -e "${GREEN}firewall-cmd --reload${NC}"
                                echo -e "\n2. 或使用sudo执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}sudo firewall-cmd --permanent --add-port=$port/tcp${NC}"
                                done
                                echo -e "${GREEN}sudo firewall-cmd --reload${NC}"
                                ;;
                            "UFW_ACTIVE")
                                echo -e "1. 使用root用户执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}ufw allow $port/tcp${NC}"
                                done
                                echo -e "\n2. 或使用sudo执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}sudo ufw allow $port/tcp${NC}"
                                done
                                ;;
                            "IPTABLES_ACTIVE")
                                echo -e "1. 使用root用户执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}iptables -A INPUT -p tcp --dport $port -j ACCEPT${NC}"
                                done
                                echo -e "${GREEN}service iptables save${NC}"
                                echo -e "\n2. 或使用sudo执行:"
                                for port in "${closed_ports[@]}"; do
                                    echo -e "${GREEN}sudo iptables -A INPUT -p tcp --dport $port -j ACCEPT${NC}"
                                done
                                echo -e "${GREEN}sudo service iptables save${NC}"
                                ;;
                        esac
                        return 1
                    else
                        log_info "节点 $node 的防火墙已启用，所需端口已开放"
                    fi
                    ;;
                "FIREWALL_INACTIVE"|"NO_FIREWALL")
                    log_info "节点 $node 的防火墙未启用"
                    ;;
                *)
                    log_warning "节点 $node 的防火墙状态检查失败: $result"
                    return 1
                    ;;
            esac
        else
            # 本地节点检查
            if command -v systemctl >/dev/null 2>&1; then
                if sudo systemctl is-active --quiet firewalld; then
                    if [ ! "$firewall_status_shown" ]; then
                        log_info "检测到防火墙(firewalld)已启用"
                        firewall_status_shown=true
                    fi
                    local closed_ports=()
                    for port in "${ports[@]}"; do
                        if ! sudo firewall-cmd --list-ports | grep -w "$port/tcp" >/dev/null 2>&1; then
                            closed_ports+=("$port")
                        fi
                    done
                    
                    if [ ${#closed_ports[@]} -gt 0 ]; then
                        log_warning "本地节点防火墙(firewalld)已启用，以下端口未开放: ${closed_ports[*]}"
                        echo -e "\n${YELLOW}请执行以下命令开放端口:${NC}"
                        echo -e "1. 使用root用户执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}firewall-cmd --permanent --add-port=$port/tcp${NC}"
                        done
                        echo -e "${GREEN}firewall-cmd --reload${NC}"
                        echo -e "\n2. 或使用sudo执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}sudo firewall-cmd --permanent --add-port=$port/tcp${NC}"
                        done
                        echo -e "${GREEN}sudo firewall-cmd --reload${NC}"
                        return 1
                    else
                        log_info "本地节点防火墙已启用，所需端口已开放"
                    fi
                elif sudo systemctl is-active --quiet ufw; then
                    if [ ! "$firewall_status_shown" ]; then
                        log_info "检测到防火墙(ufw)已启用"
                        firewall_status_shown=true
                    fi
                    local closed_ports=()
                    for port in "${ports[@]}"; do
                        if ! sudo ufw status | grep -w "$port/tcp.*ALLOW" >/dev/null 2>&1; then
                            closed_ports+=("$port")
                        fi
                    done
                    
                    if [ ${#closed_ports[@]} -gt 0 ]; then
                        log_warning "本地节点防火墙(ufw)已启用，以下端口未开放: ${closed_ports[*]}"
                        echo -e "\n${YELLOW}请执行以下命令开放端口:${NC}"
                        echo -e "1. 使用root用户执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}ufw allow $port/tcp${NC}"
                        done
                        echo -e "\n2. 或使用sudo执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}sudo ufw allow $port/tcp${NC}"
                        done
                        return 1
                    else
                        log_info "本地节点防火墙已启用，所需端口已开放"
                    fi
                fi
            elif command -v service >/dev/null 2>&1; then
                if sudo service iptables status >/dev/null 2>&1; then
                    local closed_ports=()
                    for port in "${ports[@]}"; do
                        if ! sudo iptables -L -n | grep -w "tcp dpt:$port.*ACCEPT" >/dev/null 2>&1; then
                            closed_ports+=("$port")
                        fi
                    done
                    
                    if [ ${#closed_ports[@]} -gt 0 ]; then
                        log_warning "本地节点防火墙(iptables)已启用，以下端口未开放: ${closed_ports[*]}"
                        echo -e "\n${YELLOW}请执行以下命令开放端口:${NC}"
                        echo -e "1. 使用root用户执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}iptables -A INPUT -p tcp --dport $port -j ACCEPT${NC}"
                        done
                        echo -e "${GREEN}service iptables save${NC}"
                        echo -e "\n2. 或使用sudo执行:"
                        for port in "${closed_ports[@]}"; do
                            echo -e "${GREEN}sudo iptables -A INPUT -p tcp --dport $port -j ACCEPT${NC}"
                        done
                        echo -e "${GREEN}sudo service iptables save${NC}"
                        return 1
                    else
                        log_info "本地节点防火墙已启用，所需端口已开放"
                    fi
                fi
            fi
            # 移除重复的日志
            if [ ! "$firewall_status_shown" ]; then
                log_info "本地节点防火墙未启用或所需端口已开放"
            fi
        fi
        return 0
    }
    
    local firewall_check_failed=false
    
    # 检查所有节点的防火墙状态
    if [ "$DEPLOY_MODE" = "hybrid" ]; then
        for node in "${ALL_NODES[@]}"; do
            if [ "$node" != "localhost" ] && [ "$node" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_node_firewall "$node" true; then
                    firewall_check_failed=true
                fi
            else
                if ! check_node_firewall "$node" false; then
                    firewall_check_failed=true
                fi
            fi
        done
    else
        for master in "${MASTER_IPS[@]}"; do
            if [ "$master" != "localhost" ] && [ "$master" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_node_firewall "$master" true; then
                    firewall_check_failed=true
                fi
            else
                if ! check_node_firewall "$master" false; then
                    firewall_check_failed=true
                fi
            fi
        done
        for worker in "${WORKER_IPS[@]}"; do
            if [ "$worker" != "localhost" ] && [ "$worker" != "$(hostname -I | awk '{print $1}')" ]; then
                if ! check_node_firewall "$worker" true; then
                    firewall_check_failed=true
                fi
            else
                if ! check_node_firewall "$worker" false; then
                    firewall_check_failed=true
                fi
            fi
        done
    fi
    
    if [ "$firewall_check_failed" = true ]; then
        # 转换为小写进行比较
        local action_lower=$(echo "$FIREWALL_CHECK_ACTION" | tr '[:upper:]' '[:lower:]')
        if [ "$action_lower" = "error" ]; then
            echo -e "\n${RED}检测到防火墙问题，请按上述提示处理后再次运行安装脚本${NC}"
            exit 1
        else
            log_warning "检测到防火墙配置问题，但已配置为仅警告模式，继续安装..."
        fi
    fi
}

# ============================================================================
# 执行单个步骤
# ============================================================================
execute_step() {
    local step_num="$1"
    local step_info="${INSTALL_STEPS[$step_num]}"
    local step_func="${step_info%%:*}"
    local step_desc="${step_info#*:}"
    
    if [ -z "$step_info" ]; then
        log_error "无效的步骤编号: $step_num"
        return 1
    fi
    
    log_info "[$step_num/$TOTAL_STEPS] $step_desc..."
    json_output "running" "$step_num" "$step_desc"
    save_step_status "$step_num" "running"
    
    local result=0
    case "$step_func" in
        read_config)
            read_config || result=$?
            ;;
        check_user)
            check_user || result=$?
            ;;
        check_firewall)
            check_firewall || result=$?
            ;;
        check_java)
            # 检查节点java环境
            if [ "$DEPLOY_MODE" = "hybrid" ]; then
                for node in "${ALL_NODES[@]}"; do
                    if [ "$node" != "localhost" ] && [ "$node" != "$(hostname -I | awk '{print $1}')" ]; then
                        check_java "$node" "true" || result=$?
                    else 
                        check_java "localhost" "false" || result=$?
                    fi
                done
            else
                for master in "${MASTER_IPS[@]}"; do
                    if [ "$master" != "localhost" ] && [ "$master" != "$(hostname -I | awk '{print $1}')" ]; then
                        check_java "$master" "true" || result=$?
                    else 
                        check_java "localhost" "false" || result=$?
                    fi
                done
                for worker in "${WORKER_IPS[@]}"; do
                    if [ "$worker" != "localhost" ] && [ "$worker" != "$(hostname -I | awk '{print $1}')" ]; then
                        check_java "$worker" "true" || result=$?
                    else 
                        check_java "localhost" "false" || result=$?
                    fi
                done
            fi
            ;;
        check_dependencies)
            check_dependencies || result=$?
            handle_selinux || true
            ;;
        check_memory)
            check_memory || result=$?
            ;;
        check_ports)
            check_ports || result=$?
            ;;
        handle_package)
            handle_package || result=$?
            ;;
        setup_config)
            if [ "$DEPLOY_MODE" = "hybrid" ]; then
                setup_hybrid_mode || result=$?
            else
                setup_separated_mode || result=$?
            fi
            # 配置Master HTTP端口 (SeaTunnel 2.3.9+)
            if is_seatunnel_ge_239; then
                if [ "$DEPLOY_MODE" = "separated" ]; then
                    configure_master_http_port || true
                fi
            fi
            ;;
        configure_checkpoint)
            configure_checkpoint || result=$?
            ;;
        install_plugins)
            install_plugins_and_libs || result=$?
            setup_cluster_scripts || true
            sed -i "s/root/$INSTALL_USER/g" "$SEATUNNEL_HOME/bin/seatunnel-start-cluster.sh" 2>/dev/null || true
            ;;
        distribute_nodes)
            distribute_to_nodes || result=$?
            ;;
        setup_environment)
            setup_environment || result=$?
            ;;
        setup_auto_start)
            setup_auto_start || result=$?
            ;;
        start_cluster)
            start_cluster || result=$?
            ;;
        check_services)
            check_services || result=$?
            show_completion_info || true
            ;;
        *)
            log_error "未知步骤函数: $step_func"
            result=1
            ;;
    esac
    
    if [ $result -eq 0 ]; then
        save_step_status "$step_num" "completed"
        json_output "completed" "$step_num" "$step_desc 完成"
        log_success "[$step_num/$TOTAL_STEPS] $step_desc 完成"
    else
        save_step_status "$step_num" "failed"
        json_output "failed" "$step_num" "$step_desc 失败"
        log_error "[$step_num/$TOTAL_STEPS] $step_desc 失败"
    fi
    
    return $result
}

# 主函数
main() {
    # 如果是列出步骤模式
    if [ "$LIST_STEPS" = true ]; then
        list_all_steps
        exit 0
    fi
    
    # 初始化临时文件列表
    declare -a TEMP_FILES=()
    trap cleanup_temp_files EXIT INT TERM
    
    # 检查参数冲突
    if [ "$NO_PLUGINS" = true ] && [ "$ONLY_INSTALL_PLUGINS" = true ]; then
        log_error "参数冲突: --no-plugins 和 --install-plugins 不能同时使用"
    fi
    
    # 如果是仅安装插件模式
    if [ "$ONLY_INSTALL_PLUGINS" = true ]; then
        log_info "仅安装/更新插件模式..."
        read_config
        if [ ! -d "$SEATUNNEL_HOME" ]; then
            log_error "SeaTunnel安装目录不存在: $SEATUNNEL_HOME"
        fi
        install_plugins_and_libs
        log_success "插件安装/更新完成"
        exit 0
    fi
    
    # 如果指定了--no-plugins，覆盖配置文件中的设置
    if [ "$NO_PLUGINS" = true ]; then
        INSTALL_CONNECTORS=false
    fi
    
    # 确定执行范围
    local start_step=1
    local end_step=$TOTAL_STEPS
    
    if [ -n "$RUN_STEP" ]; then
        # 仅执行指定步骤
        if ! [[ "$RUN_STEP" =~ ^[0-9]+$ ]] || [ "$RUN_STEP" -lt 1 ] || [ "$RUN_STEP" -gt $TOTAL_STEPS ]; then
            log_error "无效的步骤编号: $RUN_STEP (有效范围: 1-$TOTAL_STEPS)"
        fi
        start_step=$RUN_STEP
        end_step=$RUN_STEP
        
        # 单步执行时，如果不是第一步，需要先读取配置
        if [ "$RUN_STEP" -gt 1 ]; then
            log_info "加载配置..."
            read_config
        fi
    fi
    
    if [ -n "$STOP_AT_STEP" ]; then
        if ! [[ "$STOP_AT_STEP" =~ ^[0-9]+$ ]] || [ "$STOP_AT_STEP" -lt 1 ] || [ "$STOP_AT_STEP" -gt $TOTAL_STEPS ]; then
            log_error "无效的停止步骤: $STOP_AT_STEP (有效范围: 1-$TOTAL_STEPS)"
        fi
        end_step=$STOP_AT_STEP
    fi
    
    # 清除旧状态（完整安装时）
    if [ -z "$RUN_STEP" ] && [ "$start_step" -eq 1 ]; then
        clear_step_status
    fi
    
    # 显示执行计划
    if [ "$OUTPUT_MODE" != "json" ]; then
        echo ""
        echo "============================================"
        echo "SeaTunnel 安装向导"
        echo "============================================"
        if [ -n "$RUN_STEP" ]; then
            echo "执行步骤: $RUN_STEP"
        else
            echo "执行步骤: $start_step - $end_step"
        fi
        echo "============================================"
        echo ""
    fi
    
    # 执行步骤
    for step in $(seq $start_step $end_step); do
        if ! execute_step "$step"; then
            log_error "步骤 $step 执行失败，安装中止"
            exit 1
        fi
        
        # 检查是否需要停止
        if [ -n "$STOP_AT_STEP" ] && [ "$step" -eq "$STOP_AT_STEP" ]; then
            log_info "已执行到指定步骤 $STOP_AT_STEP，停止安装"
            log_info "继续安装请运行: ./install_seatunnel.sh --step $((step + 1))"
            break
        fi
    done
    
    if [ "$OUTPUT_MODE" != "json" ] && [ -z "$STOP_AT_STEP" ] && [ -z "$RUN_STEP" ]; then
        echo ""
        log_success "SeaTunnel 安装完成!"
    fi
}

# 执行主函数
main
