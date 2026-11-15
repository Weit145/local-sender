import { test, expect } from "@playwright/test";

test("terminal is the only style, settings persist, dialogs remain accessible", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await page.goto("/");
  await expect(page.getByText("Готово к подключению")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Устройства рядом." }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "terminal");
  expect(
    await page
      .locator("html")
      .evaluate((element) => getComputedStyle(element).backgroundColor),
  ).toBe("rgb(5, 10, 7)");
  await page.screenshot({ path: "test-results/terminal-devices.png" });
  await page.getByRole("button", { name: "Настройки", exact: true }).click();
  await expect(
    page.getByRole("group", { name: "Тема оформления" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Переключить тему" }),
  ).toHaveCount(0);
  await page.getByLabel(/Время поиска/).fill("2");
  await page.getByRole("button", { name: "Сохранить настройки" }).click();
  await expect(
    page.getByText("Настройки сохранены", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Закрыть уведомление" }).click();
  await page.screenshot({ path: "test-results/terminal-settings.png" });
  await page.reload();
  await expect(page.getByText("Готово к подключению")).toBeVisible();
  await page.getByRole("button", { name: "Настройки", exact: true }).click();
  await expect(page.getByLabel(/Время поиска/)).toHaveValue("2");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "terminal");
  await page.setViewportSize({ width: 860, height: 620 });
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBe(true);
  await page
    .getByRole("button", { name: "Устройства", exact: false })
    .first()
    .click();
  await page.getByRole("button", { name: "По IP-адресу", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Адрес устройства" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("two real instances discover, reject, accept, exchange messages and disconnect", async ({
  page,
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 1240, height: 850 },
  });
  const peer = await context.newPage();
  try {
    await peer.goto("http://127.0.0.1:19451");
    await page.goto("/");
    await expect(page.getByText("Готово к подключению")).toBeVisible();
    await expect(peer.getByText("Готово к подключению")).toBeVisible();
    await page
      .getByRole("button", { name: "Найти устройства", exact: true })
      .click();
    await expect(
      page.getByText("127.0.0.1:19461", { exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Подключиться", exact: true })
      .click();
    await expect(
      peer.getByText("Новое устройство хочет подключиться"),
    ).toBeVisible();
    await peer.getByRole("button", { name: "Отклонить", exact: true }).click();
    await expect(page.getByRole("alert")).toContainText(
      "Собеседник отклонил подключение",
    );
    await page
      .getByRole("button", { name: "Подключиться", exact: true })
      .click();
    await peer.getByRole("button", { name: "Принять", exact: true }).click();
    await expect(
      page.getByRole("textbox", { name: "Сообщение", exact: true }),
    ).toBeEnabled();
    await page
      .getByRole("textbox", { name: "Сообщение", exact: true })
      .fill("Привет из первого окна!");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(
      peer.getByText("Привет из первого окна!", { exact: true }).last(),
    ).toBeVisible();
    await peer
      .getByRole("textbox", { name: "Сообщение", exact: true })
      .fill("Всё работает — отвечаю 👋");
    await peer.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(
      page.getByText("Всё работает — отвечаю 👋", { exact: true }).last(),
    ).toBeVisible();
    if (
      await page
        .getByRole("button", { name: "Закрыть уведомление" })
        .isVisible()
    )
      await page.getByRole("button", { name: "Закрыть уведомление" }).click();
    await page.screenshot({ path: "test-results/terminal-chat.png" });
    await page.getByRole("button", { name: "Отключить устройство" }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Отключить", exact: true })
      .click();
    await expect(
      page.getByRole("textbox", { name: "Сообщение", exact: true }),
    ).toBeDisabled();
    await expect(
      peer.getByRole("textbox", { name: "Сообщение", exact: true }),
    ).toBeDisabled();
  } finally {
    await context.close();
  }
});

test("manual connection can be canceled while waiting for approval", async ({
  page,
  browser,
}) => {
  const context = await browser.newContext();
  const peer = await context.newPage();
  try {
    await peer.goto("http://127.0.0.1:19451");
    await page.goto("/");
    await expect(peer.getByText("Готово к подключению")).toBeVisible();
    await expect(page.getByText("Готово к подключению")).toBeVisible();
    await page
      .getByRole("button", { name: "По IP-адресу", exact: true })
      .click();
    await page
      .getByRole("textbox", { name: "Адрес устройства" })
      .fill("127.0.0.1:19461");
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Подключиться", exact: true })
      .click();
    await expect(
      peer.getByText("Новое устройство хочет подключиться"),
    ).toBeVisible();
    await expect(page.getByText("Ожидаем решения собеседника…")).toBeVisible();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Отмена", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(
      peer.getByText("Новое устройство хочет подключиться"),
    ).toHaveCount(0);
  } finally {
    await context.close();
  }
});
