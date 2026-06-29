/**
 * 运行环境信息采集
 *
 * 自动填充 deviceId（持久化）、sessionId（会话级）、platform、clientInfo 等，
 * 兼容浏览器与 Node（Node 下安全跳过浏览器专属 API）。
 */

import { uuid } from './utils';

const DEVICE_ID_KEY = '__uba_device_id__';
const SESSION_ID_KEY = '__uba_session_id__';

/** 判断当前是否在浏览器环境 */
export function isBrowser(): boolean {
  return typeof window !== 'undefined' && typeof document !== 'undefined';
}

// 内存级设备/会话 ID（Node 或存储不可用时回退使用）
let memDeviceId = '';
let memSessionId = '';

/**
 * 获取或生成设备 ID。
 * 浏览器：localStorage 持久化，保证同一设备稳定。
 * Node：进程级（无持久化）。
 */
export function getDeviceId(): string {
  if (!isBrowser()) {
    return memDeviceId || (memDeviceId = uuid());
  }
  try {
    const existed = window.localStorage.getItem(DEVICE_ID_KEY);
    if (existed) {
      return existed;
    }
    const id = uuid();
    window.localStorage.setItem(DEVICE_ID_KEY, id);
    return id;
  } catch {
    // localStorage 不可用（隐私模式等），回退内存级
    return memDeviceId || (memDeviceId = uuid());
  }
}

/**
 * 获取或生成会话 ID。
 * 浏览器：sessionStorage 会话级（标签页关闭即失效）。
 * Node：进程级。
 */
export function getSessionId(): string {
  if (!isBrowser()) {
    return memSessionId || (memSessionId = uuid());
  }
  try {
    const existed = window.sessionStorage.getItem(SESSION_ID_KEY);
    if (existed) {
      return existed;
    }
    const id = uuid();
    window.sessionStorage.setItem(SESSION_ID_KEY, id);
    return id;
  } catch {
    return memSessionId || (memSessionId = uuid());
  }
}

/** 探测客户端平台：web/ios/android/mini_program/node */
export function detectPlatform(): string {
  if (!isBrowser()) {
    return 'node';
  }
  const ua = (navigator.userAgent || '').toLowerCase();
  if (/iphone|ipad|ipod/.test(ua)) {
    return 'ios';
  }
  if (/android/.test(ua)) {
    return 'android';
  }
  // 小程序环境探测
  const w = window as unknown as { wx?: { getSystemInfo?: unknown } };
  if (w.wx && typeof w.wx.getSystemInfo === 'function') {
    return 'mini_program';
  }
  return 'web';
}

/** 采集 clientInfo（userAgent/referer 等），仅浏览器有值；无值返回 undefined */
export function getClientInfo(): { userAgent?: string; referer?: string } | undefined {
  if (!isBrowser()) {
    return undefined;
  }
  const info: { userAgent?: string; referer?: string } = {};
  const ua = navigator.userAgent || '';
  if (ua) {
    info.userAgent = ua;
  }
  if (document.referrer) {
    info.referer = document.referrer;
  }
  return Object.keys(info).length > 0 ? info : undefined;
}

/** 当前页面 URL（浏览器），Node 返回空 */
export function getPageUrl(): string {
  if (!isBrowser()) {
    return '';
  }
  return window.location ? window.location.href : '';
}

/** 视口宽度（像素），Node/无值返回 0 */
export function getViewportWidth(): number {
  if (!isBrowser()) {
    return 0;
  }
  return (
    window.innerWidth ||
    document.documentElement?.clientWidth ||
    document.body?.clientWidth ||
    0
  );
}

/**
 * 计算元素的 XPath（如 /html/body/div[2]/section[1]/a[3]）。
 * 用于点击热力图按元素维度聚合。非浏览器或无目标返回空串。
 */
export function buildElementXPath(target: unknown): string {
  if (!isBrowser() || !target || !(target instanceof Element)) {
    return '';
  }
  const parts: string[] = [];
  let node: Element | null = target;
  while (node && node.nodeType === 1 && node !== document.documentElement) {
    let idx = 1;
    let sib = node.previousElementSibling;
    while (sib) {
      if (sib.tagName === node.tagName) {
        idx++;
      }
      sib = sib.previousElementSibling;
    }
    const tag = node.tagName.toLowerCase();
    parts.unshift(`${tag}[${idx}]`);
    node = node.parentElement;
  }
  if (node === document.documentElement) {
    parts.unshift('html');
  }
  return parts.length > 0 ? '/' + parts.join('/') : '';
}

/**
 * 计算点击相对文档（非视口）的坐标，含滚动偏移。
 * 返回 { x, y }，单位像素；非浏览器或事件无值返回 0。
 */
export function getClickCoords(
  event: MouseEvent | PointerEvent,
): { x: number; y: number } {
  if (!isBrowser()) {
    return { x: 0, y: 0 };
  }
  const scrollX = window.pageXOffset || document.documentElement?.scrollLeft || 0;
  const scrollY = window.pageYOffset || document.documentElement?.scrollTop || 0;
  return {
    x: Math.round((event.clientX || 0) + scrollX),
    y: Math.round((event.clientY || 0) + scrollY),
  };
}
