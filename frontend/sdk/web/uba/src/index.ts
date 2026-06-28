/**
 * UBA SDK —— 对外入口
 *
 * 对接 go-wind-uba collector 服务（POST /uba/v1/report，appId+appSecret 鉴权）。
 *
 * 用法（ESM 模块）：
 *   import { UbaClient } from '@go-wind-uba/uba-sdk';
 *   const uba = UbaClient.init({ appId, appSecret, endpoint });
 *   uba.track('click', { button: 'buy' });
 *
 * 用法（浏览器）：
 *   <script type="module">
 *     import { UbaClient } from './dist/index.js';
 *     UbaClient.init({ appId, appSecret, endpoint }).track('click', { button: 'buy' });
 *   </script>
 */

export { UbaClient } from './client';
export type { PostReportResponse } from './client';

export { EventType, RiskStatus } from './types';
export type {
  UbaConfig,
  ReportEvent,
  BehaviorEvent,
  RiskEvent,
  ClientInfo,
  TrackOptions,
  PostReportRequest,
  TypeErrorDetail,
  ErrorDetail,
  KratosError,
} from './types';
