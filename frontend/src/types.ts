export interface Settings {
  tcpAddress: string;
  udpAddress: string;
  broadcastAddress: string;
  searchTimeout: number;
}
export interface NetworkInterface {
  Name: string;
  IP: string;
  Network: string;
}
export interface Device {
  IP: string;
  Address: string;
}
export type Status =
  "connecting" | "awaiting_approval" | "connected" | "disconnected";
export interface Connection {
  ID: string;
  Address: string;
  Incoming: boolean;
  Status: Status;
}
export interface ConnectionRequest {
  ID: string;
  Address: string;
}
export interface Message {
  id: number;
  connectionID: string;
  text: string;
  outgoing: boolean;
  time: string;
}
export interface State {
  revision: number;
  generation: number;
  hostname: string;
  platform: string;
  running: boolean;
  error: string;
  settings: Settings;
  interfaces: NetworkInterface[];
  devices: Device[];
  connections: Connection[];
  requests: ConnectionRequest[];
  messages: Message[];
  searching: boolean;
  connecting: string[];
}
export const initialState: State = {
  revision: 0,
  generation: 0,
  hostname: "Это устройство",
  platform: "",
  running: false,
  error: "",
  settings: {
    tcpAddress: ":8080",
    udpAddress: ":9999",
    broadcastAddress: "255.255.255.255:9999",
    searchTimeout: 3,
  },
  interfaces: [],
  devices: [],
  connections: [],
  requests: [],
  messages: [],
  searching: false,
  connecting: [],
};
export function normalize(state: State): State {
  return {
    ...state,
    interfaces: state.interfaces ?? [],
    devices: state.devices ?? [],
    connections: state.connections ?? [],
    requests: state.requests ?? [],
    messages: state.messages ?? [],
    connecting: state.connecting ?? [],
  };
}
export function host(address: string) {
  return address.startsWith("[")
    ? address.slice(1, address.indexOf("]"))
    : address.split(":")[0];
}
export function friendlyError(error: unknown): string {
  const text = error instanceof Error ? error.message : String(error);
  if (/context canceled|context cancelled/i.test(text)) return "";
  if (/deadline exceeded/i.test(text))
    return "Время ожидания истекло. Попробуйте ещё раз.";
  if (/connection rejected/i.test(text))
    return "Собеседник отклонил подключение.";
  if (/connection refused/i.test(text))
    return "Устройство не принимает подключения. Проверьте адрес и запущено ли приложение.";
  if (/already exists|already connected/i.test(text))
    return "Подключение к этому устройству уже существует.";
  if (/not running/i.test(text))
    return "Сеть недоступна. Проверьте параметры в настройках.";
  if (/invalid connection state/i.test(text))
    return "Соединение уже завершено или запрос обработан.";
  if (/invalid address/i.test(text))
    return "Укажите адрес устройства и порт, например 192.168.1.20:8080.";
  if (/single line/i.test(text)) return "Отправляйте сообщение одной строкой.";
  if (
    /Failed to fetch|NetworkError|not valid JSON|Unexpected token/i.test(text)
  )
    return "Не удалось связаться с приложением. Запустите десктопную версию.";
  return text;
}
