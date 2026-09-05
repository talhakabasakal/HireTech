/* eslint-disable @typescript-eslint/no-require-imports */
const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("desktopAPI", Object.freeze({
  getAppVersion: () => ipcRenderer.invoke("app:get-version"),
  getPlatform: () => ipcRenderer.invoke("app:get-platform"),
  getDeviceIdentity: () => ipcRenderer.invoke("device:get-identity"),
  signDeviceChallenge: (challenge) => ipcRenderer.invoke("device:sign", challenge),
  getSessionTokens: () => ipcRenderer.invoke("session:get"),
  setSessionTokens: (tokens) => ipcRenderer.invoke("session:set", tokens),
  clearSessionTokens: () => ipcRenderer.invoke("session:clear"),
}));
