# TBridge

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md)

通过电脑或手机浏览器，控制 Linux/macOS 上的持久终端会话。选择电脑和会话，发送 shell 命令，并操作 Cursor CLI、Codex CLI 等终端 AI agent。

## 功能与环境要求

- 网页：由同一个服务端提供响应式网页，内置 xterm.js 终端引擎。
- 服务端：Go 标准库、tmux 和 PTY，支持 Linux/macOS 的 amd64、arm64。
- 支持创建、重命名和结束会话，实时输出、中文输入、多行提示词、快捷内容和终端辅助按键。
- 多个客户端可以使用同一个持久 tmux 会话。客户端断线不结束任务，重连恢复当前画面及最多 1,000 行历史；电脑重启会结束会话。

TBridge 管理自己创建的 tmux 会话，不能选择电脑上任意一个已经打开的桌面 Terminal 窗口，也不能向 Cursor 编辑器的聊天框输入内容。Cursor/Codex 支持指在受管会话中运行对应 CLI。

运行服务端二进制需要在电脑上安装 **tmux 3.2+**：Ubuntu 使用 `sudo apt install tmux`，macOS 使用 `brew install tmux`。AI CLI 需在服务端用户账号下单独安装并登录。

从源码构建需要 Go 1.22+、Node.js 20+ 和 npm。服务端以普通用户运行，终端命令拥有该用户的权限。

## 构建与安装

在项目根目录执行以下命令，npm 会正常联网下载所需依赖：

~~~sh
cd web
npm ci --ignore-scripts --no-audit --no-fund
cd ..
bash scripts/build-server.sh
~~~

按服务端电脑的系统和架构选择二进制：

| 服务端系统 / 架构 | 二进制 |
| --- | --- |
| Linux x86_64 | `dist/tbridge-linux-amd64` |
| Linux aarch64 / arm64 | `dist/tbridge-linux-arm64` |
| macOS Intel | `dist/tbridge-darwin-amd64` |
| macOS Apple Silicon | `dist/tbridge-darwin-arm64` |

使用 `uname -m` 查看电脑架构。Go 不在 PATH 时可设置 `GO_BIN`。服务端构建会嵌入网页，修改网页资源后需重新构建服务端；直接构建 Go 前应执行 `bash scripts/prepare-web.sh`。

## 启动服务端

以下以 Linux x86_64 为例，其他平台请按上表替换复制的二进制路径：

~~~sh
mkdir -p "$HOME/.local/bin"
cp dist/tbridge-linux-amd64 "$HOME/.local/bin/tbridge"
chmod +x "$HOME/.local/bin/tbridge"
"$HOME/.local/bin/tbridge" serve --listen 0.0.0.0:8443
~~~

客户端使用期间保持这个终端运行。**`serve` 必须一直运行**；`--listen` 是它的监听地址参数，不是单独的命令。如需关闭终端，按后文配置后台服务。电脑需要保持联网且不休眠。

不指定 `--listen` 时默认监听 `127.0.0.1:8443`，只有本机能访问。`0.0.0.0:8443` 监听所有 IPv4 网卡，包括局域网和 Tailscale，请用防火墙或网络规则限制访问范围。客户端地址不能填写 `0.0.0.0`。

## 局域网与 Tailscale 外网连接

### 同一局域网

使用服务端电脑的局域网 IP，例如 `https://192.168.1.100:8443`，并允许目标局域网访问 TCP 8443。手机和电脑之间需要网络可达。

### 手机在外网，通过 Tailscale 连接

只要两台设备能通过 Tailscale 通信，就不需要公网 IP 或路由器端口转发。

1. 在服务端电脑和客户端设备上安装 [Tailscale](https://tailscale.com/download)。
2. 登录并连接到同一个 tailnet；如果公司管理设备审批和访问规则，请确认已允许这两台设备通信。
3. 在**服务端电脑**上查询它的 Tailscale IPv4 地址：

~~~sh
tailscale ip -4
~~~

4. 配对和浏览器访问均使用这个实际地址。后文以 **`100.99.206.33` 作为服务端示例 IP**；命令输出不同就替换为实际值，不要使用手机 IP。
5. 手机切换到移动数据或其他 Wi-Fi 后，仍需保持 Tailscale 已连接；电脑也需持续运行 Tailscale 和 TBridge。
6. 确认防火墙及 tailnet 规则允许手机访问电脑的 TCP 8443。Ubuntu 启用了 UFW 且符合网络策略时，可参考：

~~~sh
sudo ufw allow in on tailscale0 to any port 8443 proto tcp
~~~

如果只允许在 Tailscale 网卡监听，先按 Ctrl+C 停止前台服务，再使用以下命令替代监听所有网卡的命令：

~~~sh
"$HOME/.local/bin/tbridge" serve --listen 100.99.206.33:8443
~~~

绑定 Tailscale IP 前需先连接 Tailscale。TBridge 本身不提供公网中转或 NAT 穿透，设备之间的网络连接由 Tailscale 提供。

## 客户端配对与后续重连

保持服务端运行，在**服务端电脑另开一个终端**，使用同一个系统用户执行：

~~~sh
"$HOME/.local/bin/tbridge" pair --address https://100.99.206.33:8443
~~~

局域网使用时改成对应的局域网 HTTPS 地址。必须包含 `https://` 和正确端口。输出包含证书 SHA-256 指纹，以及以 `tbridge:` 开头的完整连接串。复制整条连接串，不要包含前后的说明文字。

- 配对码 **5 分钟有效，且只能成功使用一次**。
- 每个浏览器配置分别运行 `pair` 获取新码。新码会替换尚未使用的旧码，因此先完成一次配对，再生成下一个。
- 生成新码**不会撤销已经配对成功的客户端**。
- 配对成功后，客户端保存独立访问凭证。普通重连、配对码过期、服务端重启都不需要重新配对。
- 清除或过期的浏览器 Cookie、设备权限被撤销、服务端证书被替换后需要重新配对。浏览器 Cookie 目前有效期为 30 天。
- `pair` 输出后会退出，只有 `serve` 需要持续运行。有效期内的连接串应作为敏感信息保管。

### 网页客户端

1. 在客户端设备的浏览器打开 `https://100.99.206.33:8443/`，外网设备同样需要 Tailscale 网络可达。
2. 服务端会生成自签名证书。在浏览器证书详情中比对 SHA-256 指纹与本机 `pair` 输出，一致后再添加证书例外或信任。TLS 信任由浏览器处理，JavaScript 不能固定校验证书。
3. 为这个浏览器生成一个**新的、未使用过的**连接串。填写 **Device name**，将连接串粘贴到 **Connection string**，点击 **Pair browser**。
4. 点击 **New session** 创建会话，或打开已有会话；不同浏览器可以使用同一个会话。
5. 在编辑器输入 `pwd`，点击 **Insert into terminal**，再点击 **Enter**；也可以直接在终端区域用键盘输入。
6. 按需使用会话操作、**Reconnect**、辅助按键和 **Save shortcut**。**Unpair browser** 会撤销当前浏览器的访问权限。

**Enter 只发送终端回车键，不会插入编辑器草稿。** 先点击 **Insert into terminal**，再点击 **Enter**；在编辑器内按 Enter 只会换行。发送前等待 **Connected**，并确认终端提示符可见。

使用 **Refresh** 刷新会话、**Rename** 重命名、**End** 结束会话及其中任务。关闭浏览器标签页不会结束任务。手机使用同一个网页，无需安装客户端应用。

**Computers → Add** 保存其他服务端的链接，并在独立标签页打开。每个 HTTPS 来源、浏览器配置独立配对，不是跨服务端代理。网页快捷内容和电脑链接只保存在当前浏览器。认证使用 Secure、HttpOnly、SameSite=Strict Cookie，令牌不进入 JavaScript 存储或 URL 参数。

## 命令、快捷内容与 AI agent

输入发送到选定会话的**当前前台程序**，可能是 shell、agent 或确认提示。发送前检查电脑和会话名称。

使用 agent 时，先在终端切换到项目目录并执行，再启动已安装的 CLI：

~~~sh
cd /absolute/path/to/project
codex
~~~

每条命令分别通过 Insert、Enter 发送。Cursor 使用当前安装的 Cursor CLI 版本提供的命令。请先在服务端电脑安装并登录 agent；TBridge 不安装 agent、不提供 API key，也不修改 agent 授权设置。

agent 启动后，使用同一个编辑器输入提示词，并处理它的确认对话。网页端提供 Esc、Tab、Backspace、Delete、Home、End、PgUp/PgDn、方向键、Ctrl、Ctrl+C、Ctrl+D 和 **Clear (Ctrl+L)**。Backspace 发送 DEL；Clear 向前台程序发送 Ctrl+L，用于清屏或重绘，具体行为由程序决定。**Clear input** 只清空草稿，不发送任何内容。Ctrl 按钮将下一个终端字符转为控制按键。Ctrl+C 通常中断前台任务，Ctrl+D 发送 EOF，可能退出 shell。

- 多行提示词需要前台程序启用 **bracketed paste**。被阻止时使用单行输入或在程序中启用支持；编辑器内按回车只会添加换行。
- 编辑器拒绝终端控制字符，请使用辅助按键或实体键盘发送。
- 网页 **Save shortcut** 保存草稿，选择后填入编辑器，仍需分别执行 Insert 和 Enter。
- 输出是连续终端流，不是每条命令的独立结果或退出码接口。服务端确认只表示 PTY 接收了输入，不代表执行成功。
- 同一会话的多个客户端共享 shell、当前目录及输入流，同时输入需相互协调。
- 不要在网页快捷内容中保存秘密。

如果需要在电脑加入同一个会话，复制界面显示的 `tmux ... attach-session ...` 命令；网页中展开 **Open this session on the computer**。按 **Ctrl+B，再按 D** 脱离，连续按两次 Ctrl+B 可将该按键传给前台程序。不要修改 tmux 内部会话 ID，或在受管会话中添加窗口、窗格，应使用 TBridge 的会话操作。

## 设备管理与状态目录

查看授权设备，并使用输出中的设备 ID 撤销指定设备：

~~~sh
"$HOME/.local/bin/tbridge" devices
"$HOME/.local/bin/tbridge" revoke --id DEVICE_ID
~~~

撤销后活动连接约十秒内关闭。需要撤销访问权限时请在服务端撤销对应浏览器设备。

默认状态目录：Linux 为 `~/.config/tbridge`，macOS 为 `~/Library/Application Support/tbridge`。其中包含证书、设备记录、待使用配对码和 tmux socket。升级时保留该目录，所有命令使用同一个系统用户运行；任务运行期间不要删除目录。

自定义目录时，**所有命令必须传入相同的绝对路径**，例如：

~~~sh
"$HOME/.local/bin/tbridge" serve --state /absolute/path/tbridge-state --listen 0.0.0.0:8443
~~~

另一个终端执行：

~~~sh
"$HOME/.local/bin/tbridge" pair --state /absolute/path/tbridge-state --address https://100.99.206.33:8443
"$HOME/.local/bin/tbridge" devices --state /absolute/path/tbridge-state
~~~

## 后台运行服务端

在同一端口启动后台服务前，先停止前台服务。下列模板默认监听全部 IPv4 网卡，启用前检查监听地址和 PATH；将 tmux、AI CLI 的可执行文件目录加入服务 PATH。用户服务可能不会加载交互式 shell 的环境配置。

### Linux：systemd 用户服务

安装好 `~/.local/bin/tbridge` 后，在项目根目录复制模板：

~~~sh
mkdir -p "$HOME/.config/systemd/user"
cp deploy/tbridge.service "$HOME/.config/systemd/user/tbridge.service"
~~~

根据需要修改复制后的文件，限制 `--listen`、扩展 PATH，或添加自定义 `--state`，然后启用并查看：

~~~sh
systemctl --user daemon-reload
systemctl --user enable --now tbridge.service
systemctl --user status tbridge.service
journalctl --user -u tbridge.service -n 100 --no-pager
~~~

服务管理命令：

~~~sh
systemctl --user restart tbridge.service
systemctl --user stop tbridge.service
systemctl --user disable tbridge.service
~~~

启用后的用户服务随用户管理器启动，通常是在登录时。如果需要登录前启动、退出登录后仍运行，可由管理员启用 lingering：

~~~sh
sudo loginctl enable-linger "$USER"
~~~

### macOS：LaunchAgent

复制模板：

~~~sh
mkdir -p "$HOME/Library/LaunchAgents"
cp deploy/dev.tbridge.agent.plist "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

编辑复制后的文件：将**所有 `/Users/YOU`** 替换为实际用户主目录，检查 PATH（Apple Silicon Homebrew 通常位于 `/opt/homebrew/bin`），并按需修改监听地址和状态目录。然后校验并加载：

~~~sh
plutil -lint "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl print "gui/$(id -u)/dev.tbridge.agent"
~~~

LaunchAgent 在用户登录时运行，KeepAlive 会在进程退出后重启它。编辑或升级前先停止：

~~~sh
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

修改后重新用 `bootstrap` 加载。回滚时停止服务并替换二进制。现有 tmux 任务通常可以保留到 TBridge 服务重启后，但电脑重启会结束任务；保留状态目录可保留客户端凭证。

## 故障排查

| 现象 | 检查方法 |
| --- | --- |
| `Client sent an HTTP request to an HTTPS server.` | 浏览器和配对地址使用 `https://`，不要使用 `http://`。 |
| 手机无法连接 | 确认 serve 在运行、电脑未休眠、IP 属于服务端，且不是只监听 127.0.0.1；检查防火墙、端口及局域网/Tailscale 连通性。 |
| Wi-Fi 能用，移动数据不能用 | 连接手机 Tailscale，检查两台设备是否在目标 tailnet，访问规则是否允许 TCP 8443。 |
| 配对失败 | 生成新码，五分钟内使用，不要重复使用被其他客户端消耗的码；确认服务用户和状态目录一致。 |
| 证书错误 | 在电脑本地比对指纹；证书替换后核对新指纹并重新配对浏览器；不要全局关闭 TLS 校验。 |
| Connected，但 Insert 没有回显 | 用单行 `pwd` 测试并查看 Insert 后的状态：queued 表示等待确认，server accepted 仅表示 PTY 接收，不代表命令完成；renderer/input 错误表示客户端调用失败。也可尝试直接在终端输入或新建会话，并记录状态与错误。 |
| Enter 没有发送编辑器文字 | 先点 Insert（网页为 Insert into terminal），再点 Enter；Enter 本身不会发送草稿。 |
| 多行粘贴被阻止 | 需要前台程序启用 bracketed paste。 |
| 更新源码后网页没有终端 | 执行 `bash scripts/prepare-web.sh`，重新构建并重启服务端；网页资源在编译时嵌入。 |
| 找不到 agent 或未登录 | 检查服务用户的 PATH、agent 安装与登录，以及项目权限。 |
| Address already in use | 先停止占用同一端口的前台服务或后台服务。 |
| 断线或输入投递结果未知 | 重试前先检查终端；为避免重复执行，输入不会自动重放。 |

## 验证

~~~sh
cd server
go test -race -v ./...
go vet ./...
cd ..
node tests/terminal.test.cjs
node tests/browser.test.cjs
~~~

测试覆盖配对、撤销权限、浏览器 Cookie 与来源、WebSocket 帧、PTY 输入与尺寸调整及终端粘贴、按键。环境不支持 Unix socket 时，tmux 集成测试会明确跳过，需在真实服务端执行。已执行检查及待验证项目见 [VALIDATION.md](VALIDATION.md)。

macOS 运行行为、手机浏览器输入及具体 Cursor/Codex 交互仍需对应设备验收。正式使用前请人工审查远程命令执行代码。依赖许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
