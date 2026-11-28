/**
 * SPA 路由监听
 *
 * 通过包装 history.pushState/replaceState 并监听 popstate/hashchange，
 * 把前端路由变化归一为一个回调。供自动 PV 采集使用。
 */

import { isBrowser } from './context';

export interface RouteWatcherHandle {
  destroy(): void;
}

function currentRoute(): string {
  return window.location.pathname + window.location.search + window.location.hash;
}

/**
 * 监听路由变化。同一地址的连续变化不会重复触发。
 * 返回 undefined 表示当前不在浏览器环境。
 */
export function watchRouteChanges(onChange: (route: string) => void): RouteWatcherHandle | undefined {
  if (!isBrowser()) {
    return undefined;
  }

  let lastRoute = currentRoute();

  const notify = () => {
    const route = currentRoute();
    if (route === lastRoute) {
      return;
    }
    lastRoute = route;
    onChange(route);
  };

  // 包装 pushState/replaceState：路由库（vue-router/react-router）都走这两个 API
  const originalPushState = window.history.pushState.bind(window.history);
  const originalReplaceState = window.history.replaceState.bind(window.history);

  window.history.pushState = function patchedPushState(...args: Parameters<typeof originalPushState>) {
    const ret = originalPushState(...args);
    notify();
    return ret;
  } as typeof window.history.pushState;

  window.history.replaceState = function patchedReplaceState(
    ...args: Parameters<typeof originalReplaceState>
  ) {
    const ret = originalReplaceState(...args);
    notify();
    return ret;
  } as typeof window.history.replaceState;

  window.addEventListener('popstate', notify);
  window.addEventListener('hashchange', notify);

  return {
    destroy() {
      window.history.pushState = originalPushState;
      window.history.replaceState = originalReplaceState;
      window.removeEventListener('popstate', notify);
      window.removeEventListener('hashchange', notify);
    },
  };
}
