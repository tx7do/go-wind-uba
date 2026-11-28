/**
 * 小程序/小游戏运行时的定时器全局（lib ES2019 不含 DOM timers，
 * 刻意不引 DOM lib：宿主没有 document/window）。
 */
/* eslint-disable @typescript-eslint/no-explicit-any */
declare function setInterval(handler: (...args: any[]) => void, timeout: number, ...args: any[]): number;
declare function clearInterval(intervalId: number): void;
declare function setTimeout(handler: (...args: any[]) => void, timeout: number, ...args: any[]): number;
