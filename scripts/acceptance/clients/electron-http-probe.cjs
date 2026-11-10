// Optional external HTTP client for Windows capture acceptance. No product UI.
const { app, session } = require("electron");
app.setPath("userData", process.env.ARCHSCOPE_T581_USER_DATA);
app.commandLine.appendSwitch("disable-background-networking");
app.commandLine.appendSwitch("disable-component-update");
app.commandLine.appendSwitch("disable-sync");
app.disableHardwareAcceleration();
app.whenReady().then(async () => {
  const clientSession = session.fromPartition("t581-acceptance", { cache: false });
  await clientSession.setProxy({
    mode: "fixed_servers",
    proxyRules: process.env.ARCHSCOPE_T581_PROXY,
    proxyBypassRules: "<-loopback>"
  });
  const response = await clientSession.fetch(process.env.ARCHSCOPE_T581_URL);
  if (!response.ok) throw new Error("HTTP " + response.status);
  const body = await response.text();
  console.log(response.status + " " + body.length);
  app.quit();
}).catch((error) => {
  console.error(error);
  app.exit(1);
});
