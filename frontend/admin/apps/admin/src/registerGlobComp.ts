import type { App } from 'vue';

import {
  Badge,
  Button,
  Card,
  Divider,
  Dropdown,
  Empty,
  Input,
  Layout,
  Menu,
  Popconfirm,
  RangePicker,
  Select,
  SelectOption,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  TableColumn,
  Tabs,
  Tag,
  Timeline,
  TimelineItem,
  Tree,
} from 'ant-design-vue';

/**
 * 注册全局组件
 * @param app
 */
export function registerGlobComp(app: App) {
  app
    .use(Input)
    .use(Button)
    .use(Layout)
    .use(Space)
    .use(Card)
    .use(Switch)
    .use(Popconfirm)
    .use(Dropdown)
    .use(Tag)
    .use(Tabs)
    .use(Divider)
    .use(Menu)
    .use(Select)
    .use(Tree)
    .use(Table)
    .use(Statistic)
    .use(Timeline)
    .use(Empty)
    .use(Badge)
    .use(Spin);

  // 以下子组件是普通组件（非 Plugin，无 install 方法），需用 app.component 单独注册。
  // 注册名必须是组件自身 .name（带 A 前缀），模板里的 <a-range-picker> 等才能匹配。
  app
    .component('ATableColumn', TableColumn)
    .component('ATimelineItem', TimelineItem)
    .component('ARangePicker', RangePicker)
    .component('ASelectOption', SelectOption);
}
