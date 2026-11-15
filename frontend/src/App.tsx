import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import {
  ArrowDownLeft,
  ArrowRight,
  ArrowUpRight,
  Check,
  ChevronDown,
  CircleHelp,
  CircleStop,
  Laptop,
  Loader2,
  MessageCircle,
  Monitor,
  Network,
  Plus,
  Radio,
  Search,
  Send,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Wifi,
  X,
} from "lucide-react";
import { api } from "./api";
import {
  friendlyError,
  host,
  initialState,
  normalize,
  type Connection,
  type Settings,
  type State,
} from "./types";

type Page = "devices" | "messages" | "settings";
const statusNames = {
  connecting: "Подключаемся",
  awaiting_approval: "Ожидает решения",
  connected: "На связи",
  disconnected: "Отключено",
};
const platformNames: Record<string, string> = {
  linux: "Linux",
  darwin: "macOS",
  windows: "Windows",
};
const timeLabel = (time: string) =>
  new Date(time).toLocaleTimeString("ru-RU", {
    hour: "2-digit",
    minute: "2-digit",
  });

function Dialog({
  title,
  children,
  onClose,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    dialog?.querySelector<HTMLInputElement>("input")?.focus();
    return () => dialog?.close();
  }, []);
  return (
    <dialog
      ref={ref}
      aria-label={title}
      onCancel={onClose}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="dialog-inner">
        <div className="dialog-heading">
          <h2>{title}</h2>
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Закрыть"
          >
            <X size={20} />
          </button>
        </div>
        {children}
      </div>
    </dialog>
  );
}

export default function App() {
  const [state, setState] = useState<State>(initialState);
  const [ready, setReady] = useState(false);
  const [page, setPage] = useState<Page>("devices");
  const [localIP, setLocalIP] = useState("");
  const [filter, setFilter] = useState("");
  const [selectedID, setSelectedID] = useState("");
  const [draft, setDraft] = useState("");
  const [address, setAddress] = useState("");
  const [manual, setManual] = useState(false);
  const [help, setHelp] = useState(false);
  const [searched, setSearched] = useState(false);
  const [busy, setBusy] = useState<Set<string>>(new Set());
  const [toast, setToast] = useState<{ text: string; error: boolean } | null>(
    null,
  );
  const [settings, setSettings] = useState<Settings>(initialState.settings);
  const [seen, setSeen] = useState<Record<string, number>>({});
  const [disconnectID, setDisconnectID] = useState("");
  const bottom = useRef<HTMLDivElement>(null);
  const lastError = useRef("");
  const generation = useRef(0);
  const notify = useCallback((text: string, error = false) => {
    if (text) setToast({ text, error });
  }, []);
  const receive = useCallback((value: State) => {
    const next = normalize(value);
    setState((previous) =>
      next.revision > previous.revision ? next : previous,
    );
    setReady(true);
  }, []);
  const refresh = useCallback(
    async () => receive(await api.snapshot()),
    [receive],
  );

  useEffect(() => {
    let active = true;
    const off = api.subscribe((value) => {
      if (active) receive(value);
    });
    api
      .initialize()
      .then((value) => {
        if (active) receive(value);
      })
      .catch((error) => {
        if (active) {
          setReady(true);
          notify(friendlyError(error), true);
        }
      });
    const timer = window.setInterval(() => {
      if (active)
        api
          .snapshot()
          .then((value) => {
            if (active) receive(value);
          })
          .catch(() => {});
    }, 2500);
    return () => {
      active = false;
      off();
      clearInterval(timer);
    };
  }, [receive, notify]);
  useEffect(() => {
    setSettings(state.settings);
  }, [
    state.settings.tcpAddress,
    state.settings.udpAddress,
    state.settings.broadcastAddress,
    state.settings.searchTimeout,
  ]);
  useEffect(() => {
    if (!state.interfaces.some((item) => item.IP === localIP))
      setLocalIP(state.interfaces[0]?.IP ?? "");
  }, [state.interfaces, localIP]);
  useEffect(() => {
    if (generation.current !== state.generation) {
      generation.current = state.generation;
      setSelectedID("");
      setSeen({});
      setSearched(false);
    }
  }, [state.generation]);
  useEffect(() => {
    if (state.error && state.error !== lastError.current)
      notify(friendlyError(state.error), true);
    lastError.current = state.error;
  }, [state.error, notify]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 6500);
    return () => clearTimeout(timer);
  }, [toast]);
  const messages = state.messages.filter(
    (message) => message.connectionID === selectedID,
  );
  useEffect(() => {
    if (page !== "messages" || !selectedID) return;
    const latest =
      state.messages
        .filter((m) => m.connectionID === selectedID && !m.outgoing)
        .at(-1)?.id ?? 0;
    setSeen((previous) =>
      previous[selectedID] === latest
        ? previous
        : { ...previous, [selectedID]: latest },
    );
    bottom.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [state.messages, selectedID, page]);
  useEffect(() => {
    setDraft("");
  }, [selectedID]);

  const run = async <T,>(
    key: string,
    action: () => Promise<T>,
    success?: (value: T) => void,
  ) => {
    setBusy((previous) => new Set(previous).add(key));
    try {
      const result = await action();
      success?.(result);
    } catch (error) {
      notify(friendlyError(error), true);
    } finally {
      setBusy((previous) => {
        const next = new Set(previous);
        next.delete(key);
        return next;
      });
      await refresh().catch(() => {});
    }
  };
  const activeConnections = state.connections.filter(
    (item) => item.Status === "connected",
  );
  const chats = [...state.connections].sort(
    (a, b) =>
      Number(b.Status === "connected") - Number(a.Status === "connected") ||
      Number(b.ID) - Number(a.ID),
  );
  const selected = state.connections.find((item) => item.ID === selectedID);
  const unread = (id: string) =>
    state.messages.filter(
      (m) => m.connectionID === id && !m.outgoing && m.id > (seen[id] ?? 0),
    ).length;
  const totalUnread = state.connections.reduce(
    (sum, item) => sum + unread(item.ID),
    0,
  );
  const openChat = (connection: Connection) => {
    setSelectedID(connection.ID);
    setPage("messages");
  };
  const discover = () => {
    setSearched(true);
    void run("search", () => api.discover(localIP));
  };
  const connect = (addr: string) =>
    void run(
      `connect:${addr}`,
      () => api.connect(addr),
      (connection) => {
        setManual(false);
        openChat(connection);
        notify("Соединение установлено");
      },
    );
  const onlineText = !ready
    ? "Запускаемся…"
    : state.running
      ? "Готово к подключению"
      : "Сеть недоступна";
  const localName = state.hostname || "Это устройство";
  const networkChanged =
    settings.tcpAddress !== state.settings.tcpAddress ||
    settings.udpAddress !== state.settings.udpAddress ||
    settings.broadcastAddress !== state.settings.broadcastAddress ||
    settings.searchTimeout !== state.settings.searchTimeout;

  const deviceCard = (device: { Address: string; IP: string }) => {
    const connection = activeConnections.find(
      (item) => item.Address === device.Address,
    );
    const connecting =
      state.connecting.includes(device.Address) ||
      busy.has(`connect:${device.Address}`);
    return (
      <article className="device-card" key={device.Address}>
        <div className="device-top">
          <span className="device-icon">
            <Monitor size={27} strokeWidth={1.6} />
          </span>
          <span className="device-tag">
            <span className="status-dot" />В сети
          </span>
        </div>
        <h3>{device.IP}</h3>
        <p className="mono">{device.Address}</p>
        <div className="device-card-footer">
          <span>Устройство в вашей сети</span>
          {connection ? (
            <button
              className="text-button"
              onClick={() => openChat(connection)}
            >
              Открыть чат <ArrowRight size={15} />
            </button>
          ) : connecting ? (
            <button
              className="text-button"
              onClick={() =>
                void run(`cancel:${device.Address}`, () =>
                  api.cancelConnect(device.Address),
                )
              }
            >
              <Loader2 className="spin" size={15} /> Отмена
            </button>
          ) : (
            <button
              className="text-button"
              disabled={!state.running}
              onClick={() => connect(device.Address)}
            >
              Подключиться <ArrowUpRight size={17} />
            </button>
          )}
        </div>
      </article>
    );
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">
            <Radio size={25} />
          </span>
          <div>
            local<span>_sender</span>
            <small>ЛОКАЛЬНЫЕ СООБЩЕНИЯ</small>
          </div>
        </div>
        <div className="nav-label">// НАВИГАЦИЯ</div>
        <nav aria-label="Основная навигация">
          <button
            className={page === "devices" ? "nav-item active" : "nav-item"}
            onClick={() => setPage("devices")}
          >
            <Network size={19} />
            Устройства<span className="nav-count">{state.devices.length}</span>
          </button>
          <button
            className={page === "messages" ? "nav-item active" : "nav-item"}
            onClick={() => setPage("messages")}
          >
            <MessageCircle size={19} />
            Сообщения
            {totalUnread > 0 && (
              <span className="nav-count unread">{totalUnread}</span>
            )}
          </button>
          <button
            className={page === "settings" ? "nav-item active" : "nav-item"}
            onClick={() => setPage("settings")}
          >
            <SlidersHorizontal size={19} />
            Настройки
          </button>
        </nav>
        <div className="sidebar-bottom">
          <button
            className="nav-item help-button"
            onClick={() => setHelp(true)}
          >
            <CircleHelp size={18} />
            Как это работает
            <ArrowUpRight size={14} />
          </button>
          <div className="local-profile">
            <span className="profile-icon">
              <Laptop size={20} />
            </span>
            <div>
              <strong title={localName}>{localName}</strong>
              <small>
                <span
                  className={
                    state.running ? "status-dot" : "status-dot offline"
                  }
                />
                Это устройство
              </small>
            </div>
            <span className="version">v0.1</span>
          </div>
        </div>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div className="breadcrumb">
            local<span>/</span>
            <strong>
              {page === "devices"
                ? "Устройства"
                : page === "messages"
                  ? "Сообщения"
                  : "Настройки"}
            </strong>
          </div>
          <div className="topbar-right">
            <span className="network-indicator">
              <span
                className={state.running ? "status-dot" : "status-dot offline"}
              />
              {onlineText}
            </span>
          </div>
        </header>
        <main>
          {state.requests.length > 0 && (
            <section
              className="request-stack"
              aria-label="Входящие подключения"
            >
              {state.requests.map((request) => (
                <div className="request-banner" key={request.ID}>
                  <span className="request-icon">
                    <ArrowDownLeft size={22} />
                  </span>
                  <div>
                    <strong>Новое устройство хочет подключиться</strong>
                    <p>{request.Address} · Разрешить обмен сообщениями?</p>
                  </div>
                  <div className="request-actions">
                    <button
                      className="button secondary compact"
                      disabled={busy.has(`request:${request.ID}`)}
                      onClick={() =>
                        void run(`request:${request.ID}`, () =>
                          api.reject(request.ID),
                        )
                      }
                    >
                      Отклонить
                    </button>
                    <button
                      className="button primary compact"
                      disabled={busy.has(`request:${request.ID}`)}
                      onClick={() =>
                        void run(
                          `request:${request.ID}`,
                          () => api.accept(request.ID),
                          () => {
                            setSelectedID(request.ID);
                            setPage("messages");
                          },
                        )
                      }
                    >
                      <Check size={15} />
                      Принять
                    </button>
                  </div>
                </div>
              ))}
            </section>
          )}
          {ready && !state.running && (
            <div className="offline-banner">
              <Wifi size={19} />
              <div>
                <strong>Не удалось запустить локальную сеть</strong>
                <span>Проверьте порты и сетевые параметры в настройках.</span>
              </div>
              <button
                className="text-button"
                onClick={() => setPage("settings")}
              >
                Настроить <ArrowRight size={15} />
              </button>
            </div>
          )}

          {page === "devices" && (
            <div className="page devices-page">
              <div className="page-heading">
                <div>
                  <div className="eyebrow">// ОБНАРУЖЕНИЕ УСТРОЙСТВ</div>
                  <h1>
                    Устройства рядом<span className="heading-dot">.</span>
                  </h1>
                  <p>Поиск и прямое подключение в вашей сети.</p>
                </div>
                <button
                  className="button secondary"
                  onClick={() => setManual(true)}
                  disabled={!state.running}
                >
                  <Plus size={17} />
                  По IP-адресу
                </button>
              </div>
              <section className="terminal-summary" aria-label="Состояние сети">
                <div>
                  <span>ЭТО УСТРОЙСТВО</span>
                  <strong>{localName}</strong>
                </div>
                <div>
                  <span>СЕТЕВОЙ АДРЕС</span>
                  <strong>{localIP || "Не выбран"}</strong>
                </div>
                <div>
                  <span>НА СВЯЗИ</span>
                  <strong>
                    {activeConnections.length.toString().padStart(2, "0")}
                  </strong>
                </div>
              </section>
              <section
                className="discovery-section"
                aria-label="Поиск устройств"
              >
                <div className="section-title">
                  <h2>
                    Доступные устройства{" "}
                    <span className="count-pill">{state.devices.length}</span>
                  </h2>
                  <span className="section-caption">В одной сети с вами</span>
                </div>
                <div className="discovery-toolbar">
                  <div className="interface-picker">
                    <Wifi size={17} />
                    <select
                      aria-label="Сетевой интерфейс"
                      value={localIP}
                      onChange={(event) => setLocalIP(event.target.value)}
                      disabled={state.searching}
                    >
                      {state.interfaces.length === 0 && (
                        <option value="">Нет доступных интерфейсов</option>
                      )}
                      {state.interfaces.map((item) => (
                        <option value={item.IP} key={`${item.Name}-${item.IP}`}>
                          {item.Name} · {item.IP}
                        </option>
                      ))}
                    </select>
                    <ChevronDown size={14} />
                  </div>
                  <div className="search-field">
                    <Search size={16} />
                    <input
                      aria-label="Фильтр устройств"
                      placeholder="Найти по адресу…"
                      value={filter}
                      onChange={(event) => setFilter(event.target.value)}
                    />
                  </div>
                  <button
                    className="button primary search-button"
                    disabled={
                      !state.running ||
                      !localIP ||
                      (busy.has("search") && !state.searching)
                    }
                    onClick={() =>
                      state.searching
                        ? void run("cancel-search", api.cancelSearch)
                        : discover()
                    }
                  >
                    {state.searching ? (
                      <>
                        <Loader2 size={17} className="spin" />
                        Остановить поиск
                      </>
                    ) : (
                      <>
                        <Radio size={17} />
                        Найти устройства
                      </>
                    )}
                  </button>
                </div>
                {state.searching && (
                  <div className="search-progress" role="status">
                    <span />
                    Ищем устройства в вашей сети…
                  </div>
                )}
                {state.devices.length > 0 ? (
                  <div className="device-grid">
                    {state.devices
                      .filter((device) => device.Address.includes(filter))
                      .map(deviceCard)}
                    {state.devices.every(
                      (device) => !device.Address.includes(filter),
                    ) && (
                      <div className="filter-empty">
                        По этому адресу ничего не найдено.
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="empty-discovery">
                    <div
                      className={
                        state.searching
                          ? "empty-illustration searching"
                          : "empty-illustration"
                      }
                      aria-hidden="true"
                    >
                      <Radio size={30} strokeWidth={1.4} />
                    </div>
                    <h3>
                      {state.searching
                        ? "Знакомимся с вашей сетью"
                        : searched
                          ? "Пока никого не нашли"
                          : "Ожидание поиска"}
                    </h3>
                    <p>
                      {searched
                        ? "Убедитесь, что Local Sender открыт на другом устройстве\nи вы подключены к одной сети."
                        : "Откройте Local Sender на другом устройстве\nи запустите поиск в выбранной сети."}
                    </p>
                    {!state.searching && (
                      <button
                        className="text-button"
                        disabled={!state.running || !localIP}
                        onClick={discover}
                      >
                        Начать поиск <ArrowRight size={16} />
                      </button>
                    )}
                  </div>
                )}
              </section>
              <footer className="page-footer">
                <span>
                  <span
                    className={
                      state.running ? "status-dot" : "status-dot offline"
                    }
                  />
                  {localName} · {platformNames[state.platform] ?? "Desktop"}
                </span>
                <span>Локальная сеть · Без облака</span>
              </footer>
            </div>
          )}

          {page === "messages" && (
            <div className="page messages-page">
              <div className="page-heading">
                <div>
                  <div className="eyebrow">// СЕАНСЫ СВЯЗИ</div>
                  <h1>
                    Сообщения<span className="heading-dot">.</span>
                  </h1>
                  <p>Прямые сообщения между устройствами.</p>
                </div>
                <button
                  className="button secondary"
                  onClick={() => {
                    setPage("devices");
                  }}
                >
                  <Plus size={17} />
                  Новый разговор
                </button>
              </div>
              <div className="chat-layout">
                <aside className="conversation-list">
                  <div className="conversation-list-heading">
                    Ваши подключения<span>{chats.length}</span>
                  </div>
                  {chats.length === 0 ? (
                    <div className="empty-conversations">
                      <MessageCircle size={27} strokeWidth={1.3} />
                      <p>
                        Здесь появятся
                        <br />
                        ваши разговоры
                      </p>
                    </div>
                  ) : (
                    chats.map((connection) => {
                      const last = state.messages
                        .filter((m) => m.connectionID === connection.ID)
                        .at(-1);
                      return (
                        <button
                          className={
                            selectedID === connection.ID
                              ? "conversation selected"
                              : "conversation"
                          }
                          key={connection.ID}
                          onClick={() => setSelectedID(connection.ID)}
                        >
                          <span className="conversation-avatar">
                            <Monitor size={22} />
                            <i
                              className={
                                connection.Status === "connected"
                                  ? "status-dot"
                                  : "status-dot offline"
                              }
                            />
                          </span>
                          <span className="conversation-body">
                            <strong>{host(connection.Address)}</strong>
                            <small>
                              {last?.text ?? statusNames[connection.Status]}
                            </small>
                          </span>
                          {unread(connection.ID) > 0 ? (
                            <span className="unread-pill">
                              {unread(connection.ID)}
                            </span>
                          ) : (
                            last && <time>{timeLabel(last.time)}</time>
                          )}
                        </button>
                      );
                    })
                  )}
                </aside>
                {selected ? (
                  <section
                    className="chat-panel"
                    aria-label={`Чат с ${selected.Address}`}
                  >
                    <header className="chat-header">
                      <span className="conversation-avatar">
                        <Monitor size={23} />
                      </span>
                      <div>
                        <h2>{host(selected.Address)}</h2>
                        <span>
                          <span
                            className={
                              selected.Status === "connected"
                                ? "status-dot"
                                : "status-dot offline"
                            }
                          />
                          {statusNames[selected.Status]}
                        </span>
                      </div>
                      <button
                        className="icon-button disconnect-button"
                        aria-label="Отключить устройство"
                        disabled={selected.Status === "disconnected"}
                        onClick={() => setDisconnectID(selected.ID)}
                      >
                        <CircleStop size={20} />
                      </button>
                    </header>
                    <div className="message-history" aria-live="polite">
                      <div className="chat-date">Текущий сеанс</div>
                      <div className="chat-notice">
                        <ShieldCheck size={14} />
                        Сообщения передаются напрямую между устройствами
                      </div>
                      {messages.length === 0 && (
                        <div className="first-message">
                          <MessageCircle size={35} strokeWidth={1.3} />
                          <h3>Начните с «Привет»</h3>
                          <p>Теперь вы можете общаться напрямую.</p>
                        </div>
                      )}
                      {messages.map((message) => (
                        <div
                          className={
                            message.outgoing
                              ? "message-row outgoing"
                              : "message-row"
                          }
                          key={message.id}
                        >
                          <div className="message-bubble">
                            <p>{message.text}</p>
                            <span>
                              <time>{timeLabel(message.time)}</time>
                              {message.outgoing && (
                                <Check
                                  size={12}
                                  aria-label="Передано сетевому писателю"
                                />
                              )}
                            </span>
                          </div>
                        </div>
                      ))}
                      <div ref={bottom} />
                    </div>
                    <form
                      className="composer"
                      onSubmit={(event) => {
                        event.preventDefault();
                        const text = draft.trim();
                        if (!text) return;
                        void run(
                          "send",
                          () => api.send(selected.ID, text),
                          () => setDraft(""),
                        );
                      }}
                    >
                      <input
                        aria-label="Сообщение"
                        placeholder={
                          selected.Status === "connected"
                            ? "Напишите сообщение…"
                            : "Соединение завершено"
                        }
                        value={draft}
                        onChange={(event) => setDraft(event.target.value)}
                        disabled={selected.Status !== "connected"}
                        maxLength={16000}
                      />
                      <span className="composer-hint">Enter ↵</span>
                      <button
                        className="send-button"
                        type="submit"
                        aria-label="Отправить сообщение"
                        disabled={
                          !draft.trim() ||
                          selected.Status !== "connected" ||
                          busy.has("send")
                        }
                      >
                        {busy.has("send") ? (
                          <Loader2 className="spin" size={19} />
                        ) : (
                          <Send size={19} />
                        )}
                      </button>
                    </form>
                    <div className="session-note">
                      История хранится только в памяти текущего сеанса.
                    </div>
                  </section>
                ) : (
                  <section className="chat-welcome">
                    <div className="welcome-icon">
                      <MessageCircle size={42} strokeWidth={1.3} />
                      <span>
                        <Sparkles size={17} />
                      </span>
                    </div>
                    <h2>Выберите собеседника</h2>
                    <p>
                      Выберите подключение слева
                      <br />
                      или найдите новое устройство.
                    </p>
                    <button
                      className="button primary"
                      onClick={() => setPage("devices")}
                    >
                      <Network size={17} />
                      Найти собеседника
                    </button>
                  </section>
                )}
              </div>
            </div>
          )}

          {page === "settings" && (
            <div className="page settings-page">
              <div className="page-heading">
                <div>
                  <div className="eyebrow">// КОНФИГУРАЦИЯ</div>
                  <h1>
                    Настройки<span className="heading-dot">.</span>
                  </h1>
                  <p>Сетевые параметры этого устройства.</p>
                </div>
              </div>
              <form
                onSubmit={(event: FormEvent) => {
                  event.preventDefault();
                  void run(
                    "settings",
                    () => api.saveSettings(settings),
                    () => notify("Настройки сохранены"),
                  );
                }}
              >
                <section className="settings-card">
                  <div className="settings-section-title">
                    <span className="settings-section-icon">
                      <Network size={21} />
                    </span>
                    <div>
                      <h2>Сетевое подключение</h2>
                      <p>
                        Параметры обнаружения устройств и обмена сообщениями.
                      </p>
                    </div>
                  </div>
                  <div className="settings-fields">
                    <label>
                      Адрес TCP
                      <input
                        required
                        value={settings.tcpAddress}
                        onChange={(event) =>
                          setSettings({
                            ...settings,
                            tcpAddress: event.target.value,
                          })
                        }
                        placeholder=":8080"
                      />
                      <small>Адрес и порт для входящих подключений.</small>
                    </label>
                    <label>
                      Адрес UDP
                      <input
                        required
                        value={settings.udpAddress}
                        onChange={(event) =>
                          setSettings({
                            ...settings,
                            udpAddress: event.target.value,
                          })
                        }
                        placeholder=":9999"
                      />
                      <small>Порт ответов на поиск устройств.</small>
                    </label>
                    <label>
                      Адрес поиска
                      <input
                        required
                        value={settings.broadcastAddress}
                        onChange={(event) =>
                          setSettings({
                            ...settings,
                            broadcastAddress: event.target.value,
                          })
                        }
                        placeholder="255.255.255.255:9999"
                      />
                      <small>
                        Широковещательный адрес или адрес конкретного
                        устройства.
                      </small>
                    </label>
                    <label>
                      Время поиска, секунды
                      <input
                        required
                        type="number"
                        min="1"
                        max="60"
                        value={settings.searchTimeout}
                        onChange={(event) =>
                          setSettings({
                            ...settings,
                            searchTimeout: Number(event.target.value),
                          })
                        }
                      />
                      <small>
                        Сколько ждать ответы устройств: от 1 до 60 секунд.
                      </small>
                    </label>
                  </div>
                  {networkChanged && (
                    <div className="settings-warning">
                      <CircleHelp size={17} />
                      <span>
                        Применение сетевых настроек завершит текущие подключения
                        и очистит историю сообщений.
                      </span>
                    </div>
                  )}
                </section>
                <div className="settings-save">
                  <span>Настройки сохраняются на этом устройстве.</span>
                  <button
                    className="button primary"
                    type="submit"
                    disabled={busy.has("settings")}
                  >
                    {busy.has("settings") ? (
                      <Loader2 className="spin" size={17} />
                    ) : (
                      <Check size={17} />
                    )}
                    Сохранить настройки
                  </button>
                </div>
                <section className="about-card">
                  <span className="brand-mark small">
                    <Radio size={21} />
                  </span>
                  <div>
                    <strong>
                      Local Sender <span>0.1.0</span>
                    </strong>
                    <p>Устройства рядом. Сообщения напрямую.</p>
                  </div>
                  <span className="about-platform">
                    {platformNames[state.platform] ?? "Desktop"}
                  </span>
                </section>
              </form>
            </div>
          )}
        </main>
      </div>

      {manual && (
        <Dialog title="Подключиться по адресу" onClose={() => setManual(false)}>
          <div className="dialog-symbol">
            <Monitor size={29} strokeWidth={1.5} />
          </div>
          <p className="dialog-description">
            Введите IP-адрес и TCP-порт устройства.
            <br />
            Собеседник получит запрос на подключение.
          </p>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              connect(address.trim());
            }}
          >
            <label className="field-label">
              Адрес устройства
              <input
                autoFocus
                required
                aria-label="Адрес устройства"
                placeholder="192.168.1.20:8080"
                value={address}
                onChange={(event) => setAddress(event.target.value)}
                disabled={state.connecting.includes(address.trim())}
              />
            </label>
            {state.connecting.includes(address.trim()) && (
              <p className="waiting-label">
                <Loader2 size={16} className="spin" />
                Ожидаем решения собеседника…
              </p>
            )}
            <div className="dialog-buttons">
              <button
                type="button"
                className="button secondary"
                onClick={() => {
                  if (state.connecting.includes(address.trim()))
                    void run("cancel-manual", () =>
                      api.cancelConnect(address.trim()),
                    );
                  setManual(false);
                }}
              >
                Отмена
              </button>
              <button
                type="submit"
                className="button primary"
                disabled={
                  !address.trim() ||
                  state.connecting.includes(address.trim()) ||
                  busy.has(`connect:${address.trim()}`)
                }
              >
                {busy.has(`connect:${address.trim()}`) ? (
                  <Loader2 size={17} className="spin" />
                ) : (
                  <ArrowRight size={17} />
                )}
                Подключиться
              </button>
            </div>
          </form>
        </Dialog>
      )}
      {disconnectID && (
        <Dialog
          title="Завершить подключение?"
          onClose={() => setDisconnectID("")}
        >
          <p className="dialog-description">
            Обмен сообщениями с этим устройством завершится. Переписка останется
            доступна до перезапуска сети или приложения.
          </p>
          <div className="dialog-buttons">
            <button
              className="button secondary"
              onClick={() => setDisconnectID("")}
            >
              Оставить
            </button>
            <button
              className="button danger"
              disabled={busy.has("disconnect")}
              onClick={() =>
                void run(
                  "disconnect",
                  () => api.disconnect(disconnectID),
                  () => setDisconnectID(""),
                )
              }
            >
              Отключить
            </button>
          </div>
        </Dialog>
      )}
      {help && (
        <Dialog title="Всего три шага" onClose={() => setHelp(false)}>
          <div className="help-steps">
            {[
              {
                Icon: Wifi,
                title: "Одна сеть",
                text: "Подключите устройства к одному Wi-Fi или локальной сети.",
              },
              {
                Icon: Search,
                title: "Найдите друг друга",
                text: "Запустите Local Sender на обоих устройствах и нажмите «Найти устройства».",
              },
              {
                Icon: MessageCircle,
                title: "Начните разговор",
                text: "Подключитесь, дождитесь подтверждения и отправьте первое сообщение.",
              },
            ].map(({ Icon, title, text }, index) => (
              <div className="help-step" key={title}>
                <span>
                  <Icon size={20} />
                </span>
                <div>
                  <small>ШАГ 0{index + 1}</small>
                  <h3>{title}</h3>
                  <p>{text}</p>
                </div>
              </div>
            ))}
          </div>
          <p className="help-footnote">
            Автопоиск может быть ограничен настройками сети или брандмауэром. В
            таком случае подключитесь по IP-адресу.
          </p>
        </Dialog>
      )}
      <div className="toast-region" aria-live="polite">
        {toast && (
          <div
            className={toast.error ? "toast error" : "toast"}
            role={toast.error ? "alert" : "status"}
          >
            {toast.error ? <CircleHelp size={20} /> : <Check size={20} />}
            <span>{toast.text}</span>
            <button
              className="icon-button"
              onClick={() => setToast(null)}
              aria-label="Закрыть уведомление"
            >
              <X size={17} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
