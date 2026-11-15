import { Call, Events } from "@wailsio/runtime";
import type { Connection, Settings, State } from "./types";
const call = <T>(method: string, ...args: unknown[]): Promise<T> =>
  Call.ByName(
    `github.com/Weit145/local-sender/internal/desktop.App.${method}`,
    ...args,
  );
export const api = {
  initialize: () => call<State>("Initialize"),
  snapshot: () => call<State>("Snapshot"),
  discover: (ip: string) => call<void>("Discover", ip),
  cancelSearch: () => call<void>("CancelSearch"),
  connect: (addr: string) => call<Connection>("Connect", addr),
  cancelConnect: (addr: string) => call<void>("CancelConnect", addr),
  accept: (id: string) => call<void>("Accept", id),
  reject: (id: string) => call<void>("Reject", id),
  disconnect: (id: string) => call<void>("Disconnect", id),
  send: (id: string, text: string) => call<void>("Send", id, text),
  saveSettings: (settings: Settings) => call<void>("SaveSettings", settings),
  subscribe: (callback: (state: State) => void) =>
    Events.On("sender:state", (event) => callback(event.data as State)),
};
