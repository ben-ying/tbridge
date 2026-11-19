# TBridge

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md)

透過電腦或手機瀏覽器，控制 Linux/macOS 上的持久終端工作階段。選擇電腦及工作階段，傳送 shell 指令，並操作 Cursor CLI、Codex CLI 等終端 AI agent。

## 功能與環境需求

- 網頁：由同一個伺服器提供響應式網頁，內建 xterm.js 終端引擎。
- 伺服器：Go 標準函式庫、tmux 和 PTY，支援 Linux/macOS 的 amd64、arm64。
- 支援建立、重新命名及結束工作階段，即時輸出、中文輸入、多行提示詞、快捷內容和終端輔助按鍵。
- 多個用戶端可以使用同一個持久 tmux 工作階段。用戶端斷線不結束工作，重新連線恢復目前畫面及最多 1,000 行歷史；電腦重新啟動會結束工作階段。

TBridge 管理自己建立的 tmux 工作階段，無法選取電腦上任意一個已開啟的桌面 Terminal 視窗，也無法向 Cursor 編輯器的聊天框輸入內容。Cursor/Codex 支援指在受管理的工作階段中執行對應 CLI。

執行伺服器執行檔需要在電腦上安裝 **tmux 3.2+**：Ubuntu 使用 `sudo apt install tmux`，macOS 使用 `brew install tmux`。AI CLI 需在伺服器使用者帳號下另外安裝並登入。

從原始碼建置需要 Go 1.22+、Node.js 20+ 和 npm。伺服器以一般使用者執行，終端指令擁有該使用者的權限。

## 建置與安裝

在專案根目錄執行以下指令，npm 會正常連網下載所需相依套件：

~~~sh
cd web
npm ci --ignore-scripts --no-audit --no-fund
cd ..
bash scripts/build-server.sh
~~~

依伺服器電腦的系統和架構選擇執行檔：

| 伺服器系統 / 架構 | 執行檔 |
| --- | --- |
| Linux x86_64 | `dist/tbridge-linux-amd64` |
| Linux aarch64 / arm64 | `dist/tbridge-linux-arm64` |
| macOS Intel | `dist/tbridge-darwin-amd64` |
| macOS Apple Silicon | `dist/tbridge-darwin-arm64` |

使用 `uname -m` 查看電腦架構。Go 不在 PATH 時可設定 `GO_BIN`。伺服器建置會嵌入網頁，修改網頁資源後需重新建置伺服器；直接建置 Go 前應執行 `bash scripts/prepare-web.sh`。

## 啟動伺服器

以下以 Linux x86_64 為例，其他平台請依上表替換複製的執行檔路徑：

~~~sh
mkdir -p "$HOME/.local/bin"
cp dist/tbridge-linux-amd64 "$HOME/.local/bin/tbridge"
chmod +x "$HOME/.local/bin/tbridge"
"$HOME/.local/bin/tbridge" serve --listen 0.0.0.0:8443
~~~

用戶端使用期間保持這個終端執行。**`serve` 必須持續執行**；`--listen` 是它的監聽位址參數，不是獨立指令。如需關閉終端，請依後文設定背景服務。電腦需要保持連網且不休眠。

不指定 `--listen` 時預設監聽 `127.0.0.1:8443`，只有本機能存取。`0.0.0.0:8443` 監聽所有 IPv4 網路介面，包含區域網路和 Tailscale，請以防火牆或網路規則限制存取範圍。用戶端位址不能填寫 `0.0.0.0`。

## 區域網路與 Tailscale 外網連線

### 同一區域網路

使用伺服器電腦的區域網路 IP，例如 `https://192.168.1.100:8443`，並允許目標區域網路存取 TCP 8443。手機和電腦之間需要網路可達。

### 手機在外網，透過 Tailscale 連線

只要兩台裝置能透過 Tailscale 通訊，就不需要公網 IP 或路由器連接埠轉送。

1. 在伺服器電腦和用戶端裝置上安裝 [Tailscale](https://tailscale.com/download)。
2. 登入並連線到同一個 tailnet；如果公司管理裝置核准及存取規則，請確認已允許這兩台裝置通訊。
3. 在**伺服器電腦**上查詢它的 Tailscale IPv4 位址：

~~~sh
tailscale ip -4
~~~

4. 配對和瀏覽器存取均使用這個實際位址。後文以 **`100.99.206.33` 作為伺服器範例 IP**；指令輸出不同就替換為實際值，不要使用手機 IP。
5. 手機切換到行動數據或其他 Wi-Fi 後，仍需保持 Tailscale 已連線；電腦也需持續執行 Tailscale 和 TBridge。
6. 確認防火牆及 tailnet 規則允許手機存取電腦的 TCP 8443。Ubuntu 啟用了 UFW 且符合網路政策時，可參考：

~~~sh
sudo ufw allow in on tailscale0 to any port 8443 proto tcp
~~~

如果只允許在 Tailscale 介面監聽，先按 Ctrl+C 停止前景服務，再使用以下指令取代監聽所有介面的指令：

~~~sh
"$HOME/.local/bin/tbridge" serve --listen 100.99.206.33:8443
~~~

綁定 Tailscale IP 前需先連線 Tailscale。TBridge 本身不提供公網中繼或 NAT 穿透，裝置之間的網路連線由 Tailscale 提供。

## 用戶端配對與後續重新連線

保持伺服器執行，在**伺服器電腦另開一個終端**，使用同一個系統使用者執行：

~~~sh
"$HOME/.local/bin/tbridge" pair --address https://100.99.206.33:8443
~~~

區域網路使用時改成對應的區域網路 HTTPS 位址。必須包含 `https://` 和正確連接埠。輸出包含憑證 SHA-256 指紋，以及以 `tbridge:` 開頭的完整連線字串。複製整條連線字串，不要包含前後的說明文字。

- 配對碼 **5 分鐘內有效，且只能成功使用一次**。
- 每個瀏覽器設定檔分別執行 `pair` 取得新碼。新碼會取代尚未使用的舊碼，因此先完成一次配對，再產生下一個。
- 產生新碼**不會撤銷已經配對成功的用戶端**。
- 配對成功後，用戶端儲存獨立存取憑證。一般重新連線、配對碼過期、伺服器服務重新啟動都不需要重新配對。
- 清除或過期的瀏覽器 Cookie、裝置權限遭撤銷、伺服器憑證被替換後需要重新配對。瀏覽器 Cookie 目前有效期為 30 天。
- `pair` 輸出後會結束，只有 `serve` 需要持續執行。有效期內的連線字串應作為敏感資訊保管。

### 網頁用戶端

手機頁面優先顯示終端，電腦和工作階段清單放在後面。開啟工作階段會回到頁面頂端，初次聚焦終端不會向下捲動；手機上可點選 **Sessions & computers** 返回清單。Esc、Tab 等輔助鍵收進 **Terminal keys** 面板；長文字輸入框、Insert/Enter、Clear input 和已儲存快捷內容收進 **Edit prompt** 面板，兩者預設摺疊，按需展開。頂部移除副標題並縮小高度，工作階段名稱與連線狀態顯示在同一行。

瀏覽器在沒有待回覆心跳時每 5 秒傳送一次應用層心跳，等待相符回覆最多 15 秒。頁面在前景且計時器正常運作時，網路靜默中斷通常會在 15–20 秒內被發現，頁面顯示 Disconnected 並停用輸入；背景分頁可能因瀏覽器限制計時器而延後發現。網路恢復後點選 **Reconnect**，傳遞結果未知的輸入不會自動重播。

1. 在用戶端裝置的瀏覽器開啟 `https://100.99.206.33:8443/`，外網裝置同樣需要 Tailscale 網路可達。
2. 伺服器會產生自簽憑證。在瀏覽器憑證詳細資料中比對 SHA-256 指紋與本機 `pair` 輸出，一致後再新增憑證例外或信任。TLS 信任由瀏覽器處理，JavaScript 無法固定驗證憑證。
3. 為這個瀏覽器產生一個**新的、未使用過的**連線字串。填寫 **Device name**，將連線字串貼至 **Connection string**，點選 **Pair browser**。
4. 點選 **New session** 建立工作階段，或開啟既有工作階段；不同瀏覽器可以使用同一個工作階段。
5. 在編輯器輸入 `pwd`，點選 **Insert into terminal**，再點選 **Enter**；也可以直接在終端區域用鍵盤輸入。
6. 視需要使用工作階段操作、**Reconnect**、輔助按鍵和 **Save shortcut**。**Unpair browser** 會撤銷目前瀏覽器的存取權限。

**Enter 只傳送終端 Enter 鍵，不會插入編輯器草稿。** 先點選 **Insert into terminal**，再點選 **Enter**；在編輯器內按 Enter 只會換行。傳送前等待 **Connected**，並確認終端提示符號可見。

使用 **Refresh** 重新整理工作階段、**Rename** 重新命名、**End** 結束工作階段及其中工作。關閉瀏覽器分頁不會結束工作。手機使用同一個網頁，無需安裝用戶端應用程式。

**Computers → Add** 儲存其他伺服器的連結，並在獨立分頁開啟。每個 HTTPS 來源、瀏覽器設定檔獨立配對，不是跨伺服器代理。網頁快捷內容和電腦連結只保存在目前瀏覽器。驗證使用 Secure、HttpOnly、SameSite=Strict Cookie，權杖不進入 JavaScript 儲存空間或 URL 參數。

### 日夜模式、裝置清單和複製

點選頂部 **Light mode / Dark mode**，網頁和終端會一起切換日夜模式。首次跟隨系統外觀，手動選擇儲存在目前瀏覽器；切換不會清空終端輸出或重新建立連線。

配對後的 **Devices** 面板顯示已配對瀏覽器名稱、裝置 ID、Online/Offline、目前瀏覽器，以及本次伺服器啟動後記錄的最近活動時間，每 10 秒重新整理。Online 表示最近 30 秒內有驗證請求；斷網或背景休眠後可能短暫保持 Online。同一憑證下的多個分頁算一個裝置。這裡顯示已配對用戶端，不是區域網路或 Tailscale 的全部電腦；不會回傳驗證權杖或雜湊。

電腦上可先選取終端文字，再點選 **Copy text**；如果前景程式接管滑鼠，按住 Shift 再選取。未選取文字時，**Copy text** 會將目前畫面和可用歷史輸出放進可選取的純文字視窗，手機也可直接使用。點選 **Copy to clipboard**，或選取其中一部分使用瀏覽器複製操作；剪貼簿權限不可用時仍可手動選取複製。複製不會傳送終端輸入，Ctrl+C 仍是終端中斷鍵。可複製範圍為繪製器最多 1,000 行歷史及目前畫面，不是無限指令紀錄。

## 指令、快捷內容與 AI agent

輸入傳送到選定工作階段的**目前前景程式**，可能是 shell、agent 或確認提示。傳送前檢查電腦和工作階段名稱。

使用 agent 時，先在終端切換到專案目錄並執行，再啟動已安裝的 CLI：

~~~sh
cd /absolute/path/to/project
codex
~~~

每條指令分別透過 Insert、Enter 傳送。Cursor 使用目前安裝的 Cursor CLI 版本提供的指令。請先在伺服器電腦安裝並登入 agent；TBridge 不安裝 agent、不提供 API key，也不修改 agent 授權設定。

agent 啟動後，使用同一個編輯器輸入提示詞，並處理它的確認對話。網頁端提供 Esc、Tab、Backspace、Delete、Home、End、PgUp/PgDn、方向鍵、Ctrl、Ctrl+C、Ctrl+D 和 **Clear (Ctrl+L)**。Backspace 傳送 DEL；Clear 向前景程式傳送 Ctrl+L，用於清除畫面或重新繪製，具體行為由程式決定。**Clear input** 只清空草稿，不傳送任何內容。Ctrl 按鈕將下一個終端字元轉為控制按鍵。Ctrl+C 通常中斷前景工作，Ctrl+D 傳送 EOF，可能結束 shell。

- 多行提示詞需要前景程式啟用 **bracketed paste**。遭阻擋時使用單行輸入或在程式中啟用支援；編輯器內按 Enter 只會新增換行。
- 編輯器拒絕終端控制字元，請使用輔助按鍵或實體鍵盤傳送。
- 網頁 **Save shortcut** 儲存草稿，選擇後填入編輯器，仍需分別執行 Insert 和 Enter。
- 輸出是持續的終端資料流，不是每條指令的獨立結果或結束代碼介面。伺服器確認只表示 PTY 接收了輸入，不代表執行成功。
- 同一工作階段的多個用戶端共用 shell、目前目錄及輸入流，同時輸入需互相協調。
- 請勿在網頁快捷內容中儲存機密。

如需在電腦加入同一個工作階段，複製畫面顯示的 `tmux ... attach-session ...` 指令；網頁中展開 **Open this session on the computer**。按 **Ctrl+B，再按 D** 離開連接，連按兩次 Ctrl+B 可將該按鍵傳給前景程式。請勿修改 tmux 內部工作階段 ID，或在受管理的工作階段中新增視窗、窗格，應使用 TBridge 的工作階段操作。

## 裝置管理與狀態目錄

查看授權裝置，並使用輸出中的裝置 ID 撤銷指定裝置：

~~~sh
"$HOME/.local/bin/tbridge" devices
"$HOME/.local/bin/tbridge" revoke --id DEVICE_ID
~~~

撤銷後作用中的連線約十秒內關閉。需要撤銷存取權限時請在伺服器撤銷對應瀏覽器裝置。

預設狀態目錄：Linux 為 `~/.config/tbridge`，macOS 為 `~/Library/Application Support/tbridge`。其中包含憑證、裝置紀錄、待使用配對碼和 tmux socket。升級時保留該目錄，所有指令使用同一個系統使用者執行；工作執行期間請勿刪除目錄。

自訂目錄時，**所有指令必須傳入相同的絕對路徑**，例如：

~~~sh
"$HOME/.local/bin/tbridge" serve --state /absolute/path/tbridge-state --listen 0.0.0.0:8443
~~~

另一個終端執行：

~~~sh
"$HOME/.local/bin/tbridge" pair --state /absolute/path/tbridge-state --address https://100.99.206.33:8443
"$HOME/.local/bin/tbridge" devices --state /absolute/path/tbridge-state
~~~

## 背景執行伺服器

在同一連接埠啟動背景服務前，先停止前景服務。下列範本預設監聽全部 IPv4 網路介面，啟用前檢查監聽位址和 PATH；將 tmux、AI CLI 的執行檔目錄加入服務 PATH。使用者服務可能不會載入互動式 shell 的環境設定。

### Linux：systemd 使用者服務

安裝好 `~/.local/bin/tbridge` 後，在專案根目錄複製範本：

~~~sh
mkdir -p "$HOME/.config/systemd/user"
cp deploy/tbridge.service "$HOME/.config/systemd/user/tbridge.service"
~~~

視需要修改複製後的檔案，限制 `--listen`、擴充 PATH，或新增自訂 `--state`，然後啟用並查看：

~~~sh
systemctl --user daemon-reload
systemctl --user enable --now tbridge.service
systemctl --user status tbridge.service
journalctl --user -u tbridge.service -n 100 --no-pager
~~~

服務管理指令：

~~~sh
systemctl --user restart tbridge.service
systemctl --user stop tbridge.service
systemctl --user disable tbridge.service
~~~

啟用後的使用者服務隨使用者管理員啟動，通常是在登入時。如果需要登入前啟動、登出後仍執行，可由管理員啟用 lingering：

~~~sh
sudo loginctl enable-linger "$USER"
~~~

### macOS：LaunchAgent

複製範本：

~~~sh
mkdir -p "$HOME/Library/LaunchAgents"
cp deploy/dev.tbridge.agent.plist "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

編輯複製後的檔案：將**所有 `/Users/YOU`** 替換為實際使用者家目錄，檢查 PATH（Apple Silicon Homebrew 通常位於 `/opt/homebrew/bin`），並視需要修改監聽位址和狀態目錄。然後驗證並載入：

~~~sh
plutil -lint "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl print "gui/$(id -u)/dev.tbridge.agent"
~~~

LaunchAgent 在使用者登入時執行，KeepAlive 會在處理程序結束後重新啟動它。編輯或升級前先停止：

~~~sh
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

修改後重新用 `bootstrap` 載入。回復時停止服務並替換執行檔。既有 tmux 工作通常可以保留到 TBridge 服務重新啟動後，但電腦重新啟動會結束工作；保留狀態目錄可保留用戶端憑證。

### macOS：LaunchDaemon（登入前啟動）

需要開機後無須登入就執行時，使用 LaunchDaemon。透過 `UserName` 指定一般使用者執行服務。此方法與上面的 LaunchAgent 二選一；切換前先用上面的 `bootout` 指令停止 LaunchAgent，避免連接埠衝突。

在專案根目錄，以實際執行 TBridge 的使用者執行以下指令。重用現有範本並寫入絕對路徑，因為 launchd 不會展開 plist 內的 `$HOME`：

~~~sh
cp deploy/dev.tbridge.agent.plist "$HOME/dev.tbridge.daemon.plist"
plutil -replace Label -string dev.tbridge.daemon "$HOME/dev.tbridge.daemon.plist"
plutil -replace ProgramArguments.0 -string "$HOME/.local/bin/tbridge" "$HOME/dev.tbridge.daemon.plist"
plutil -insert UserName -string "$(id -un)" "$HOME/dev.tbridge.daemon.plist"
plutil -insert EnvironmentVariables.HOME -string "$HOME" "$HOME/dev.tbridge.daemon.plist"
plutil -insert WorkingDirectory -string "$HOME" "$HOME/dev.tbridge.daemon.plist"
~~~

檢查產生檔案中的 PATH，確保包含 tmux 和 AI CLI，並視需要修改 `--listen` 或新增自訂 `--state`。驗證後，將設定以 root 擁有權安裝到系統目錄並立即載入：

~~~sh
plutil -lint "$HOME/dev.tbridge.daemon.plist"
sudo install -o root -g wheel -m 644 "$HOME/dev.tbridge.daemon.plist" /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo launchctl enable system/dev.tbridge.daemon
sudo launchctl bootstrap system /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo launchctl print system/dev.tbridge.daemon
~~~

此後每次開機都會啟動，範本中的 KeepAlive 會在處理程序結束後重新啟動它。如果啟用了 FileVault，需要先解鎖啟動磁碟，macOS 才能啟動此服務。手動重新啟動：

~~~sh
sudo launchctl kickstart -k system/dev.tbridge.daemon
~~~

停止並取消開機啟動：

~~~sh
sudo launchctl bootout system /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo launchctl disable system/dev.tbridge.daemon
~~~

修改產生的設定或升級後，先卸載正在執行的服務，再使用上面的 `install`、`enable` 和 `bootstrap` 指令重新安裝、啟用並載入。

#### 啟動失敗：tmux 已安裝，但服務找不到

如果錯誤日誌提示 `tmux is required; install tmux before starting TBridge`，可能是服務的 PATH 缺少 Homebrew 目錄。LaunchDaemon 不會載入 `.zshrc` 或 `.bashrc`，終端機能找到 tmux 不代表服務也能找到。典型現象是 `state = spawn scheduled`、`last exit code = 1`、反覆出現 `Successfully spawned` / `service inactive`，且 8443 連接埠沒有監聽。

先在一般終端機檢查：

~~~sh
command -v tmux
~~~

沒有輸出時先執行 `brew install tmux`。如果輸出 `/opt/homebrew/bin/tmux`（Apple Silicon）或 `/usr/local/bin/tmux`（Intel Homebrew），需要把對應目錄明確加入服務 PATH。專案範本已包含這兩個目錄，但舊設定或手寫設定可能遺漏。

下面使用本節的 `dev.tbridge.daemon` 標籤和 plist 檔名。**如果使用了先前的 `local.tbridge` 範例，請將下面所有 `dev.tbridge.daemon` 替換為 `local.tbridge`。** 如果 tmux 安裝在其他位置，也要將其所在目錄加入 PATH：

~~~sh
sudo plutil -replace EnvironmentVariables.PATH \
  -string "$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
  /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo plutil -lint /Library/LaunchDaemons/dev.tbridge.daemon.plist
~~~

驗證顯示 `OK` 後，卸載並重新載入設定；只執行 `kickstart` 不會重新讀取修改後的 plist：

~~~sh
sudo launchctl bootout system /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo launchctl bootstrap system /Library/LaunchDaemons/dev.tbridge.daemon.plist
sudo launchctl print system/dev.tbridge.daemon
sudo lsof -nP -iTCP:8443 -sTCP:LISTEN
~~~

正常應顯示 `state = running`，並有 TBridge 監聽 8443。請同步更新來源 plist 中的 PATH，避免後續重新安裝時覆蓋修正。如果仍啟動失敗，查看 `StandardErrorPath` 指定的檔案；先前的 `local.tbridge` 範例使用 `$HOME/Library/Logs/tbridge.error.log`。

## 疑難排解

| 現象 | 檢查方法 |
| --- | --- |
| `Client sent an HTTP request to an HTTPS server.` | 瀏覽器和配對位址使用 `https://`，不要使用 `http://`。 |
| 手機無法連線 | 確認 serve 正在執行、電腦未休眠、IP 屬於伺服器，且不是只監聽 127.0.0.1；檢查防火牆、連接埠及區域網路/Tailscale 連通性。 |
| Wi-Fi 能用，行動數據不能用 | 連線手機 Tailscale，檢查兩台裝置是否在目標 tailnet，存取規則是否允許 TCP 8443。 |
| 配對失敗 | 產生新碼，五分鐘內使用，不要重複使用被其他用戶端消耗的碼；確認服務使用者和狀態目錄一致。 |
| 憑證錯誤 | 在電腦本機比對指紋；憑證替換後核對新指紋並重新配對瀏覽器；請勿全域關閉 TLS 驗證。 |
| Connected，但 Insert 沒有回顯 | 用單行 `pwd` 測試並查看 Insert 後的狀態：queued 表示等待確認，server accepted 僅表示 PTY 接收，不代表指令完成；renderer/input 錯誤表示用戶端呼叫失敗。也可嘗試直接在終端輸入或建立新工作階段，並記錄狀態與錯誤。 |
| Enter 沒有傳送編輯器文字 | 先點選 Insert（網頁為 Insert into terminal），再點選 Enter；Enter 本身不會傳送草稿。 |
| 多行貼上遭阻擋 | 需要前景程式啟用 bracketed paste。 |
| 更新原始碼後網頁沒有終端 | 執行 `bash scripts/prepare-web.sh`，重新建置並重新啟動伺服器；網頁資源在編譯時嵌入。 |
| 找不到 agent 或未登入 | 檢查服務使用者的 PATH、agent 安裝與登入，以及專案權限。 |
| Address already in use | 先停止佔用同一連接埠的前景服務或背景服務。 |
| 斷線或輸入傳遞結果未知 | 重試前先檢查終端；為避免重複執行，輸入不會自動重播。 |

## 驗證

~~~sh
cd server
go test -race -v ./...
go vet ./...
cd ..
node tests/terminal.test.cjs
node tests/browser.test.cjs
~~~

測試涵蓋配對、撤銷權限、瀏覽器 Cookie 與來源、WebSocket 框架、PTY 輸入與尺寸調整及終端貼上、按鍵。環境不支援 Unix socket 時，tmux 整合測試會明確略過，需在真實伺服器執行。已執行檢查及待驗證項目見 [VALIDATION.md](VALIDATION.md)。

macOS 執行行為、手機瀏覽器輸入及實際 Cursor/Codex 互動仍需對應裝置驗收。正式使用前請人工審查遠端指令執行程式碼。相依套件授權條款見 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
