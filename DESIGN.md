---
name: "PrismFlow · 棱流"
description: "以暖黑玻璃与香槟橙组织单服务器连接工作台。"
colors:
  primary: "#e5ab7d"
  primary-hover: "#f0bb91"
  bg: "#0b0b0b"
  text: "#f4f2ef"
  muted: "#b5afa7"
  line: "#ffffff14"
  surface: "#252522"
  solid: "#1b1b19"
  input: "#ffffff08"
  tint: "#e5ab7d18"
  danger: "#f1a09a"
  light-primary: "#9b5428"
  light-primary-hover: "#80401c"
  light-bg: "#f4f0ea"
  light-text: "#29251f"
  light-muted: "#6b6259"
  light-line: "#5f49321c"
  light-surface: "#fffdf9"
  light-solid: "#faf7f1"
  light-input: "#ffffffa8"
  light-tint: "#eadccf"
  light-danger: "#ac3937"
  chart-upload: "#a6bfa9"
  online: "#72c29a"
  status-active: "#78cda1"
  light-status-active: "#26734d"
typography:
  display:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "46px"
    lineHeight: 1.4
    fontWeight: 700
    letterSpacing: "-1.5px"
  headline:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "22px"
    fontWeight: 700
    letterSpacing: "-0.7px"
  title:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "15px"
    fontWeight: 650
    letterSpacing: "-0.3px"
  body:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "14px"
    lineHeight: 1.7
  label:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "12px"
    fontWeight: 550
  metric:
    fontFamily: "'Manrope Variable', 'Microsoft YaHei', 'PingFang SC', sans-serif"
    fontSize: "30px"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-1.2px"
  mono:
    fontFamily: "ui-monospace, monospace"
rounded:
  field: "8px"
  option: "9px"
  surface: "12px"
  modal: "14px"
  result: "7px"
  navigation: "22px"
  button: "24px"
  circle: "50%"
spacing:
  field-gap: "6px"
  option-gap: "7px"
  control-gap: "8px"
  mobile-section: "10px"
  label-gap: "12px"
  mobile-gutter: "12px"
  compact-section: "13px"
  card-gap: "14px"
  section: "16px"
  settings-section: "18px"
  desktop-gutter: "20px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.bg}"
    rounded: "{rounded.button}"
    padding: "7px 13px"
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
  button-secondary:
    backgroundColor: "{colors.input}"
    textColor: "{colors.text}"
    rounded: "{rounded.button}"
    padding: "7px 13px"
  button-text:
    textColor: "{colors.primary}"
    padding: "6px 0"
  button-icon:
    backgroundColor: "{colors.input}"
    textColor: "{colors.muted}"
    rounded: "{rounded.circle}"
    size: "31px"
  input-field:
    backgroundColor: "{colors.input}"
    textColor: "{colors.text}"
    rounded: "{rounded.field}"
    padding: "9px 11px"
  navigation-active:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.bg}"
    rounded: "{rounded.navigation}"
    padding: "9px 13px"
  surface:
    textColor: "{colors.text}"
    rounded: "{rounded.surface}"
    padding: "16px"
  resource-card:
    rounded: "{rounded.surface}"
    padding: "16px"
  protocol-option:
    backgroundColor: "{colors.input}"
    textColor: "{colors.text}"
    rounded: "{rounded.option}"
    padding: "10px 12px"
  modal:
    backgroundColor: "{colors.solid}"
    textColor: "{colors.text}"
    rounded: "{rounded.modal}"
    width: "min(660px, calc(100vw - 32px))"
  modal-wide:
    width: "min(760px, calc(100vw - 32px))"
  status-badge:
    textColor: "{colors.status-active}"
---

# Design System: PrismFlow · 棱流

## Overview

**Creative North Star: "暖黑棱镜工作台"**

暖黑背景承托半透明暖灰玻璃，香槟橙只在品牌容器、当前导航和主要操作上形成清晰落点。低饱和琥珀折光、细边框和柔和阴影让面板具备可见层次；文字、状态和真实数据始终优先于材质表现。

视觉方向沿用用户指定的 Glassmorphism Admin Panel 参考：顶部品牌栏与横向玻璃导航、内凹角资源卡、圆形工具按钮。PrismFlow 保留原创玻璃棱镜与多路流线 SVG 标志；简体中文为默认语言。本轮以更紧凑的间距和字号提高管理密度，创建与编辑集中在原生弹窗，页面只保留当前任务需要的说明。

**Key Characteristics:**
- 暖黑与香槟橙形成主层级，浅色主题使用暖白与深棕。
- 桌面横向导航和四列资源卡，移动端抽屉与两列资源卡。
- 模糊、不透明度和饱和度可实时调整，偏好保存在当前浏览器。
- 保存后自动校验并应用，数据与空状态均来自真实服务状态。
- 紧凑列表承载日常管理，新增弹窗承载输入、检测与就地错误反馈。

规范依据 `web/src/style.css`、`web/src/responsive.css`、`web/src/compact.css`、`web/src/Modal.tsx`、`web/src/ui.tsx` 与现有页面组件。样式加载顺序为基础、响应式、紧凑覆盖；同等优先级以最后的 `compact.css` 为准，高优先级组件选择器继续生效。已对照 `docs/screenshots/overview.png`、`protocols.png`、`mobile-protocols.png`、`mobile.png`、`light.png`，并核对本轮 `ordinary.png` 与 `native-listener.png`；本轮设置、手机设置、订阅和字段帮助分别对照 `settings.png`、`mobile-settings.png`、`subscription.png`、`field-help.png`。前置 token 是规范值，正文解释用途；深色为初次访问默认主题，`light-*` 对应浅色覆盖。

## Colors

这是一套暖中性色配单一橙色强调的系统；绿色仅承担上传曲线和在线状态的语义。

### Primary

- **香槟橙**（`primary`）：当前导航胶囊、主要按钮、链接、焦点与下载曲线。悬停使用 `primary-hover`，轻提示与选中选项使用 `tint`。
- **焙茶棕**（`light-primary`）：浅色主题中的对应强调色；保持与暖白底的文字辨识度。

### Neutral

- **暖黑底**（`bg`）：工作台背景与资源卡凹角的露底。浅色对应 `light-bg`。
- **暖灰玻璃**（`surface`）：参与透明混色的面板基色；它不是最终不透明画面的截图取色。浅色对应 `light-surface`。
- **实心暖灰**（`solid`）：抽屉、选项底色、通知及玻璃关闭时的表面。浅色对应 `light-solid`。
- **暖白正文 / 石灰说明**（`text`、`muted`）：分别用于主要信息与次级说明；浅色对应 `light-text`、`light-muted`。
- **微光边线 / 控件薄层**（`line`、`input`）：一像素边框、分隔线、控件底与表格悬停；浅色使用相应 `light-*`。

### Semantic

- **错误红**（`danger`、`light-danger`）：错误文本、错误提示与删除操作。
- **灰绿上传**（`chart-upload`）：上传曲线及图例，与主色下载曲线区分。
- **在线绿**（`online`）：内核运行指示点。状态标签使用 `status-active` / `light-status-active`；停用标签使用 `muted`。

**The 单一强调 Rule.** 新页面使用现有橙色主层级；状态绿用于状态或图表，不扩展为绿色侧栏或主导航。

## Typography

**Display Font / Body Font:** Manrope Variable；中文依次回退 Microsoft YaHei、PingFang SC 和系统 sans-serif。端口、路径等需要对齐的信息使用 ui-monospace、monospace。

字形紧凑清晰，英文品牌保留较强字重，中文说明依靠大小、间距和颜色建立层级。除登录叙事外，不把管理页面做成大标题展示页。

### Hierarchy

- **Display**（`display`）：仅登录叙事，手机隐藏这段叙事。
- **Headline**（`headline`）：页面标题；手机缩至（20px）。标题下的描述行在紧凑界面隐藏。
- **Title**（`title`）：常规节标题；弹窗标题为（16px），手机为（15px）。普通未包含在节标题容器中的 h2 仍沿用基础字级。
- **Body**（`body`）：页面基础字级；段落行高用于阅读内容，操作与表格按各组件紧凑排布。
- **Label**（`label`）：表单标签；按钮为同字级（600），导航为同字级，当前项加强至（750）。辅助信息常用（10–12px），仍保留实际文字标签。
- **Metric**（`metric`）：资源卡主值；手机缩至（27px）。数值采用等宽数字，上传下载读数使用（22px）。

## Layout

页面容器居中，最大宽度（1600px），桌面左右内边距使用 `desktop-gutter`；品牌栏高（64px），横向玻璃导航声明最小高度（52px）。主内容与导航相隔（18px），标题区下间距（16px）。导航同时是玻璃表面，当前最终内边距继承紧凑表面的（16px），实际高度由内容与内边距共同决定。

资源卡为四列等宽，间距使用 `card-gap`，最小高度（140px）。总览中段按（1.6fr : 1fr）组成图表与连接设置，右列最小宽度（270px）。总览下段与设置首行保持两列，服务器流量及伪装网站在设置页各占整行，间距（14px）；普通面板内边距使用 `section`，设置面板为（18px）。图表桌面高度（130px）。表格表头内边距（12px 16px）、单元格（13px 16px），保持横向滚动容器。

新增与编辑都从页面工具栏打开弹窗，不将长表单展开在列表上方。快速创建的四个单选预设使用单列列表，可转为普通创建继续调整；普通创建字段可使用两列，按表单或原生参数模式展示相关内容。统一域名设置保留紧凑的行内配置：默认地址、默认证书、同步现有入口选项和保存动作。

- **中等桌面**（761–1180px）：保留横向导航与四列资源卡；基础响应式规则的紧凑图标、导航字号继续生效，不再使用旧快速组合卡网格。
- **手机**（≤760px）：页面左右边距（12px），品牌栏高（58px），导航声明最小高度（46px）、最终表面内边距（13px）。资源卡两列，间距（10px）、最小高度（132px）；总览其余工作区、设置、网站及弹窗表单为单列。普通面板内边距（13px），资源卡（12px），主要总览区块间距（10px）。手机图表也采用最终紧凑覆盖的（130px）高度。
- **抽屉**：手机导航从左侧展开，宽（275px），配遮罩；打开时将背景主体设为不可交互。桌面没有常驻侧栏。
- **窄屏边界**：页面最小宽度（320px）。外观面板手机宽度为 `min(330px, calc(100vw - 32px))`；操作弹窗的移动尺寸另见 Components。
## Elevation & Depth

深度由背景琥珀光晕、面板混色、背板模糊和细边框共同建立。普通表面的背景为 `color-mix(in srgb, var(--surface) var(--glass-opacity), transparent)`，滤镜为 `blur(var(--glass-blur)) saturate(var(--glass-saturation))`；不是实心灰色卡片加孤立发光。

材质默认模糊（20px）、不透明度（72%）、饱和度（125%）；范围分别为（0–40px）、（45–100%）、（50–180%）。调节即时生效，恢复默认回到这三个值；只有当前浏览器保存偏好。`data-glass=off` 已有实心表面降级样式，不应声称界面中有单独的关闭开关。

### Shadow Vocabulary

- **普通玻璃表面**：深色 `0 12px 35px -25px #000`，浅色 `0 12px 35px -25px #23170c55`。
- **外观面板**：`0 15px 50px #0005`，建立浮层层级；其背景使用实心色的（95%）混色。
- **结果通知**：`0 10px 35px #0004`，位置固定在底部中央。
- **操作弹窗**：`0 22px 70px #0008`，实心暖灰底；原生 backdrop 使用 `#0009` 与（5px）模糊，将注意力留给当前操作。

交互颜色变化使用（0.15s），加载旋转周期（1s）；不加入滚动编排或持续装饰动画。系统启用减少动画时关闭动画、过渡与平滑滚动。

## Shapes

普通面板使用 `surface` 圆角，输入框使用 `field`，协议选择行使用 `option`，弹窗使用 `modal`。导航和按钮是横向胶囊，工具按钮、协议图标采用圆形。细边框统一由 `line` 提供。

资源卡是本系统的标志形状：普通圆角面板右上角留出内凹区域，桌面凹口为（52 × 52px）、内转角（16px），动作按钮随紧凑工具按钮为（31px）。手机凹口为（38 × 38px）、内转角（13px），动作按钮为（29px）。凹口露底跟随主题背景；它用于资源卡，不要求每个面板都开凹口。

## Components

### Buttons

主要按钮使用橙色与背景色反字，最小高度（34px），内边距见前置 token。次按钮使用薄层底与细边框；文本按钮使用主色，悬停出现下划线。圆形工具按钮常规与表格行内（31px），手机品牌栏因专用规则为（32px）；手机资源卡动作为（29px）。检测和端口工具按钮进一步收紧到最小高度（30px）、字级（11px）。图标按钮必须有明确的中文可访问名称。

按钮、链接和摘要的键盘焦点为主色（2px）轮廓，外偏移（4px）。禁用按钮降低不透明度至（0.45）并显示不可操作指针；不要只靠颜色表示禁用。

### Inputs / Fields

输入、选择与文本域使用控件薄层、细边框和 `field` 圆角。焦点将边框改为主色，并绘制主色（13%）混色的（3px）外环；文本插入点在 input、textarea 和可编辑内容中跟随主色；根节点的静态文本插入点透明。标题、品牌和导航文字不可选中，交互焦点仍按原规范可见。字段标签位于控件上方，间隔（6px），字段下间距（13px）。Field 优先保留显式 placeholder，否则按字段名称的统一指导补齐输入提示，适用文本输入最终回退为“请输入字段名”；标签始终保留，placeholder 不代替名称。错误信息使用错误红及实际文本。


有解释的字段在标签旁显示圆形问号按钮（22px），图标（14px）；默认次级色，展开时使用主色。按钮提供“解释 字段名”中文名称、aria-expanded 与 aria-controls。帮助默认收起，展开后以 role=note 放在标签和输入框之间，参与正常文档流，不使用绝对定位或覆盖层；细边框、控件薄层、圆角（8px）、内边距（9px 11px）、字级（12px）、行高（1.7），长内容允许任意断行。输入控件通过 aria-describedby 关联帮助。字段内 Escape 先收起帮助并阻止向外传播，避免第一次按键就关闭整个编辑弹窗。
### Navigation

桌面导航位于品牌栏下方的完整玻璃表面，首项统一为“总览”，不再另列工作台标题或“概览”入口；顶栏移除 P 头像，保留外观、主题与退出工具。主导航为总览、协议入口、用户与订阅、域名与证书、系统设置；伪装网站移入系统设置，不再占据独立导航项。未选项使用次级文字，悬停变为正文色与控件薄层，当前页使用主色胶囊并带 `aria-current`。手机抽屉打开时聚焦关闭按钮，Tab 在抽屉内循环，Escape、遮罩或选择页面均可关闭；关闭后恢复原焦点，关闭中的抽屉不可聚焦。

### Cards / Containers

普通面板内边距使用 `section`，总览资源卡桌面（16px）、手机（12px）。主值上下间距（12px），底部分隔与说明间距（10px）。面板中标题、工具和内容自然分组，不增加多层包裹。资源卡展示真实的入口、用户、证书数量与服务器周期流量；周期卡同时显示网卡状态及额度或“未设额度”，右上工具可进入系统设置。代理图表继续使用所选时段采样，两者不混称。

### Modal / Focus

新增用户、新增协议、证书关联或申请等新增操作统一使用原生 `<dialog>`；Editor 也复用 Modal，编辑不再展开在页面上。通过 `showModal()` 进入顶层，标题使用 `aria-labelledby`，包含显式“关闭弹窗”工具按钮。浏览器负责模态焦点边界，初始焦点由原生对话框算法决定；通常 Escape、关闭按钮和面板边界外的遮罩点击均关闭，卸载后恢复打开前的焦点；字段帮助展开时 Escape 先收起帮助，确认请求进行中则保持弹窗直到请求完成。

常规宽度为 `min(660px, calc(100vw - 32px))`，快速与普通创建宽版为 `min(760px, calc(100vw - 32px))`；最大高度 `calc(100dvh - 48px)`，内容可纵向滚动且阻止滚动串联。桌面弹窗外层无内边距，标题内边距（14px 18px）、正文（16px 18px）。标题栏吸顶，表单操作区吸底；表单错误在弹窗内容中显示，不被遮罩留在页面背后。

手机弹窗宽 `calc(100vw - 20px)`、最大高度 `calc(100dvh - 24px)`，常规与宽版统一；标题内边距（11px 13px），正文（13px）。当前手机表面覆盖还为 dialog 外层增加（13px）内边距，组件预览按此实际层叠呈现；表单按钮占满可用宽度。浮层保留实心底与清晰边框，不让背后的玻璃干扰填写。

### Protocol Choices / Assignment

快速创建按固定顺序展示四个单选预设：**VLESS + REALITY + Vision → VLESS + XHTTP + TLS → AnyTLS → Hysteria2**。每次只选择一个，默认 REALITY；每行包含主色圆形 radio（17px）、完整协议名、等宽端口与选中对勾。行间距（7px），行内边距（10px 12px）；选中使用主色（65%）混色边框与 `tint` 底，不使用协议复选组合或旧三列预设卡。

进入快速创建即获取四个随机五位端口，提交所选的一项；支持“重新随机端口”。等待或失败在弹窗内反馈。新入口的随机端口避开已配置和服务器本机占用；普通创建也支持随机端口，新增端口范围（10000–65535），编辑既有入口仍接受（1–65535），保留已有 443 等配置。快速弹窗中的“转为普通创建”直接保留所选协议、端口、域名、证书、REALITY 目标及用户分配，不要求重新填写。

普通创建提供“协议类型”与“配置方式”。表单模式支持 VLESS、Hysteria2、TUIC、Trojan、AnyTLS、Shadowsocks、VMess 七种协议，按能力展示传输、安全、证书、SNI、路径等字段。四个快速预设只是常用入口，不限制普通新建的协议范围。地址默认继承统一域名，当前创建可覆盖；TLS 需要证书，快速创建无证书时提供“先设置域名与证书”跳转。

原生参数模式覆盖当前内核的二十种监听器，在普通创建中输入单个 YAML / JSON 对象；名称、类型、端口由表单管理，不可在原生对象中覆盖，TUN 不展示监听端口。参数编辑区使用现有等宽字形（12px）、行高（1.6）与制表宽度（2）。认证、TLS 等由管理员自行配置，界面不显示用户分配组，也不生成用户订阅；入口列表的用户列显示“自行管理”。原生变更保存前使用实际 Mihomo 校验，未安装内核时明确提示先安装，暂时停用也不跳过校验。

REALITY 握手目标使用下拉框展示十二个候选与“自定义目标…”；只有选择自定义时才出现文本输入。“检测当前”检测当前目标，“比较候选”按四个一批请求并显示进度，结果先排可用项、再按服务器实测延迟排序。检测期间及不通过的项不可选；基础 TLS 1.3 / h2 通过后仍保留客户端实际连接验证提示。

快速创建与普通表单模式提供紧凑的“分配用户”复选组；快速创建未分配时说明暂不监听，提交文字为“创建入口”，分配后为“创建并启用”。用户编辑弹窗仍显示可用的托管协议、全选、清空与逐项勾选；无入口时在字段组内引导创建。用户分配的复选语义不与快速预设的单选语义混用。
### Badges / Charts / Empty States

状态标签采用小圆点加文字；停用使用次级色。图表使用细实线、浅虚线网格和带文字图例；没有采样时显示“尚无流量记录”及下一步说明，不生成装饰曲线、成功率或占位业务数字。零值是实际零值，空状态是尚无记录，两者按实际数据分别呈现。

证书相关说明只保留影响当前选择的内容，系统路径、自动签发与 PEM 来源在弹窗中显式选择。REALITY 不要求用户自行签发证书；TLS 选择无可用证书时给出直接跳转入口，避免把签发步骤强加给所有协议。

### Subscription / Confirmation

订阅详情在宽版 Modal 中显示，标题为“用户名 的订阅”；桌面左侧二维码、右侧链接与复制操作，手机单列。只读链接聚焦后选中内容，节点分享链接按需展开；保留含访问凭据的分享说明。订阅与通用节点链接中的节点名统一为“用户名 · 入口名”。

重置订阅链接与重置用户凭据使用 ConfirmDialog，明确区分：前者令旧订阅地址失效、保留已导入节点凭据；后者令旧地址及节点凭据失效，Shadowsocks 共享入口密码保持不变。弹窗使用现有 Modal、次按钮“取消”与主色确认按钮，按钮间距（8px），说明字级（13px）、行高（1.8）。请求中显示忙碌状态、禁用取消并阻止关闭；失败留在确认弹窗内，成功后关闭并刷新。内核停止与恢复上次配置也复用此模式，明确说明连接中断或重启影响，不把所有其他操作都描述成已迁移的自定义确认框。

### Endpoint Check

入口行提供“检测”操作并打开 EndpointCheck 弹窗；默认等待用户点击开始，过程中显示忙碌，结束后可重新检测。结果展示“可用 / 未通过 / 尚不能判断”、具体消息、逐步检查、检测时间，成功时显示延迟；错误就地显示。配置或内核运行状态变化后清除旧结果，避免沿用旧成功结论。

检测复用当前生效的真实地址、SNI、证书校验与已分配用户凭据，通过临时 Mihomo 客户端发起真实请求，不直连回退，也不重启在线主内核。原生模式自行管理客户端参数，返回尚不能判断；未分配用户或配置未生效等前置条件同样用具体原因说明。界面清楚区分服务器侧链路通过与外部客户端公网验收。

### Server Traffic / Website Settings

系统设置的服务器流量卡为全宽面板，顶部显示“全用户共享周期用量”、已用值 / 额度或“不限额”、当前网卡或不可用原因与带时区的下次重置时间。读数（22px），次级信息（11px），间隔（8px），进度条高（6px）、主色；进度最多绘至额度，不截断已用文本，不暗示超额会停用。

配置字段为每月流量上限（GiB）、每月重置日、重置时区、计量方向、监控网卡；桌面两列、手机单列，沿用现有 Field 帮助与保存反馈。重置按所选时区每月当天 00:00，无该日取月末；修改日期或时区可能开始新周期。网卡为空时使用 Linux 默认 IPv4 路由接口；Windows / 缺少 /proc 时展示不可用原因。此项是服务器全用户共享的周期网卡累计，独立于代理历史采样；额度只是展示，未实现强制断流或用户独立计费。

订阅 Header 的 upload / download / total 与此共享统计一致，仅出站时 download 为 0；重置时间在 X-PrismFlow-Traffic-Reset-At 中，不作为 expire。统计不可用时不伪造成功状态或把保留累计当作当前采集值。

伪装网站在系统设置中以独立面板展示启用复选框；开启后显示模板、标题、介绍及上传动作，使用说明按需展开。关闭仅令网站根路径返回 404，不影响订阅或 ACME；有关联 Hysteria2 网站回落时先关闭回落。上传网站走 Modal，完成后维持当前开关状态。
### Panel Version / Updates

系统设置在内核卡片之后、服务器流量之前展示全宽“面板版本与更新”卡片，直接复用 Surface、Badge、Button 和 ConfirmDialog。标题右侧标明“原生版本 / Docker 部署 / 源码运行”，定义列表展示当前版本与最新稳定版链接。卡片下间距和定义列表下间距均为（14px），说明与操作消息上间距（12px），沿用现有边框、圆角、字级、主色及手机排版；不新增更新专属视觉系统。

“检查面板版本”使用次按钮；“更新面板”使用主按钮。只有原生 Linux 部署且检测到可用新版本时可提交更新；忙碌、请求已提交、下载或应用中禁用更新按钮，已提交时文字改为“已提交更新”。检查过程显示忙碌，失败就地反馈；卡片状态按后台轮询刷新。源码与 Docker 展示相应部署指导，不显示原生自动更新选项，原生更新按钮不可用。

手动更新先使用现有确认弹窗，明确目标稳定版、面板及代理连接短暂中断、可能重新登录、失败尝试回退。原生部署的自动更新复选框注明“每小时检查，默认关闭”；开启先确认重启影响及不会自动升级 Mihomo，关闭直接异步保存。请求中禁用复选框并保留确认弹窗的忙碌约束。提交成功使用结果通知，下载、应用或失败消息保留在卡片，不把“已提交”描述为“已完成升级”。

界面只暴露受控操作，不提供任意下载 URL、目标路径或命令输入。非 root 主面板与独立特权更新服务的职责保留在产品与部署说明中。当前源码构建显示 dev；最新稳定版来自正式 Release，未发布时不生成假版本。对照截图 `docs/screenshots/updates.png`、`mobile-updates.png` 时，必须标明其中的版本与原生部署状态是 Windows 下 API mock，不能推导出真实 Release 已发布或原生升级已验收。
### Appearance / Feedback

外观面板是右上角非模态浮层；打开时聚焦关闭按钮，Escape 可关闭，卸载后恢复打开前的焦点。三个滑块带名称、当前值与单位，恢复默认是局部操作。顶部保留主题切换按钮。

保存和后台操作在当前页面反馈结果：通知使用 `role=status`，错误留在相关表单或页面；模态操作的失败必须在当前弹窗内可见。顶栏状态区明确显示“保存即生效”或“配置已保存 · 等待内核”；新增页面继续此语义，失败保留原配置，不引入全局发布步骤。

## Do's and Don'ts

### Do:
- **Do** 保持暖黑、香槟橙、横向玻璃导航与原创棱镜标志的已确认方向。
- **Do** 用真实数据、零值和说明完整的空状态表达服务状态。
- **Do** 为圆形工具按钮提供中文名称，为表单、模态弹窗、抽屉和外观面板保留清晰键盘焦点。
- **Do** 让用户看见协议分配、证书来源和保存反馈，并延续保存后自动应用的流程。
- **Do** 同时检查深色、浅色和手机布局；手机资源卡两列，其余主要工作区单列。
- **Do** 以紧凑样式的最终层叠值扩展组件；新增使用弹窗，说明靠近当前字段与操作。

### Don't:
- **Don't** 恢复绿色常驻侧栏，或以另一套配色替代已确定的参考方向。
- **Don't** 用假流量、演示成功率或装饰曲线填充真实管理页面。
- **Don't** 引入全局发布按钮，或把证书签发变成所有协议的必经步骤。
- **Don't** 恢复页面内展开的新建长表单、旧三列预设卡或重复的大段解释。
- **Don't** 让玻璃滤镜遮蔽正文、焦点、错误或当前选中状态。
- **Don't** 把参考项目的品牌标志替换进 PrismFlow，或让每个面板都重复资源卡凹角。
