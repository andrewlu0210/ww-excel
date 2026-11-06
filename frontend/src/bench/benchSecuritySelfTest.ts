// Exercise backend validation through generated Wails bindings.
import { benchApi } from '../bridge/sheet';

export async function benchSecuritySelfTest(expect: (name: string, ok: boolean, detail: string) => void): Promise<void> {
  for (const [name, call] of [
    ['負的事件數', () => benchApi.emitBurst(-1, 0)],
    ['過大的區塊數', () => benchApi.emitBurst(1, 1_000_000)],
    ['過大的事件總量', () => benchApi.emitBurst(1_000, 16)],
    ['過大的公式鏈', () => benchApi.recalcChain(100_001)],
  ] as const) {
    let message = '';
    try { await call(); } catch (error) { message = String(error); }
    expect(`測試 bindings 安全：拒絕${name}`, !!message, message);
  }
  const ping = await benchApi.ping();
  expect('測試 bindings 安全：合法探測仍可使用', ping > 0, String(ping));
  const n = await benchApi.recalcChain(1);
  expect('測試 bindings 安全：拒絕後可繼續執行', n >= 0, String(n));
}
