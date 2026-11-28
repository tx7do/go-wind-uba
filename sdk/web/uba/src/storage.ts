/**
 * 离线队列持久化
 *
 * 把未上报的事件队列镜像到 localStorage：页面崩溃、断网、重试耗尽后，
 * 下次打开自动补发。所有操作 fail-safe —— 隐私模式/容量超限时静默降级为纯内存。
 */

import type { ReportEvent } from './types';
import { isBrowser } from './context';

const STORAGE_KEY = '__uba_offline_queue__';

/** 持久化队列默认上限（条）：超过后丢最旧的 */
export const DEFAULT_MAX_PERSISTED_EVENTS = 500;

export function loadPersistedEvents(): ReportEvent[] {
  if (!isBrowser()) {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      window.localStorage.removeItem(STORAGE_KEY);
      return [];
    }
    return parsed.filter((e): e is ReportEvent => !!e && typeof e === 'object' && typeof e.eventName === 'string');
  } catch {
    return [];
  }
}

export function persistEvents(events: ReportEvent[], maxEvents: number): void {
  if (!isBrowser() || events.length === 0) {
    return;
  }
  try {
    // 超限时保留最新的
    const trimmed = events.length > maxEvents ? events.slice(events.length - maxEvents) : events;
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed));
  } catch {
    // 容量满/隐私模式：放弃持久化，不影响主流程
  }
}

export function clearPersistedEvents(): void {
  if (!isBrowser()) {
    return;
  }
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    // 忽略
  }
}
