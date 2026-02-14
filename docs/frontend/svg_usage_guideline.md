# OneBook AI SVG 使用规范

## 1. 目的
- 统一前端 SVG 使用方式，避免“同类图标多种写法”导致维护成本上升。
- 保证交互图标在不同状态下颜色、尺寸、可访问性行为一致。
- 在保证性能的前提下，保留开发期可读性与可调试性。

## 2. 使用原则（按场景选型）

### 2.1 交互图标（按钮/状态图标）
- 使用：`sprite + <use>`
- 典型场景：发送、语音、用户菜单、导航操作图标。
- 约束：
  - SVG path 使用 `fill="currentColor"`，由外层 CSS 控制颜色。
  - 图标大小由容器类控制，不在 SVG 内写死宽高。

### 2.2 展示类图形（Logo/插画/大图）
- 使用：`<img src="*.svg" />`
- 典型场景：品牌 Logo、功能示意图、流程插画。
- 约束：
  - 这类资源不依赖状态变色，不强制 `currentColor`。

## 3. 当前项目约定
- Chat 页面交互图标统一走外部 sprite：
  - `/icons/chat/sprite.svg#chat-profile`
  - `/icons/chat/sprite.svg#chat-voice`
  - `/icons/chat/sprite.svg#chat-send`
- `sprite.svg` 放在 `frontend/public/icons/chat/sprite.svg`。
- 说明：`<symbol>` 精灵文件本身直接打开通常是“空白”，需通过 `<use>` 引用渲染。

## 4. 目录与命名
- 交互 icon sprite：`frontend/public/icons/<domain>/sprite.svg`
- 单图资源：`frontend/src/assets/icons/...`、`frontend/src/assets/brand/...`
- 命名建议：
  - `kebab-case`
  - 语义优先（如 `chat-send`、`chat-voice`）

## 5. 代码示例

```tsx
const CHAT_ICON_SPRITE_URL = '/icons/chat/sprite.svg'

<svg viewBox="0 0 20 20" aria-hidden="true" className="chatgpt-send-icon">
  <use href={`${CHAT_ICON_SPRITE_URL}#chat-send`} fill="currentColor" />
</svg>
```

## 6. 可访问性
- 纯装饰图标：加 `aria-hidden="true"`。
- 交互语义放在按钮本身：`<button aria-label="发送提示">`。
- 不在图标上单独放可读文本，避免重复朗读。

## 7. 样式规范
- 按钮状态颜色由 CSS 控制，不在 SVG 文件内写死颜色（除非明确要求固定色）。
- 推荐规则：
  - 默认/悬停/禁用状态使用容器颜色切换。
  - 图标跟随 `currentColor` 自动变化。

## 8. 评审检查清单
- 新增图标是否属于“交互图标”还是“展示图形”。
- 交互图标是否使用了 `currentColor`。
- 是否补充了 `aria-label` / `aria-hidden`。
- 是否与现有域（chat/login/home）目录一致。

