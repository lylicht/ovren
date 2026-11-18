use serde::{Deserialize, Serialize};
#[cfg(target_os = "windows")]
use tauri::Manager;
use tauri_plugin_shell::{process::CommandEvent, ShellExt};

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AdapterSnapshot {
    pub name: String,
    pub connected: bool,
    pub download_bytes_per_second: f64,
    pub upload_bytes_per_second: f64,
    pub active_flows: i64,
    pub total_flows: i64,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Snapshot {
    pub running: bool,
    pub enabled: bool,
    pub since: Option<String>,
    pub adapters: Vec<AdapterSnapshot>,
}

fn state_args(app: &tauri::AppHandle) -> Result<Vec<String>, String> {
    #[cfg(any(target_os = "macos", target_os = "linux"))]
    {
        let _ = app;
        Ok(Vec::new())
    }
    #[cfg(target_os = "windows")]
    {
        let directory = app
            .path()
            .app_cache_dir()
            .map_err(|_| "Could not prepare OvrenNET storage.".to_string())?
            .join("OvrenNET");
        Ok(vec![
            "-state-dir".into(),
            directory.to_string_lossy().into_owned(),
        ])
    }
}

pub async fn snapshot(app: &tauri::AppHandle) -> Result<Snapshot, String> {
    let mut args = vec!["snapshot".to_string()];
    args.extend(state_args(app)?);
    let output = app
        .shell()
        .sidecar("ovrenNET")
        .map_err(|_| "OvrenNET is unavailable on this device.".to_string())?
        .args(args)
        .output()
        .await
        .map_err(|_| "Could not read OvrenNET status.".to_string())?;
    if !output.status.success() {
        return Err("Could not read OvrenNET status.".into());
    }
    serde_json::from_slice(&output.stdout).map_err(|_| "OvrenNET returned invalid status.".into())
}

pub async fn start(app: &tauri::AppHandle) -> Result<(), String> {
    let owner_pid = std::process::id().to_string();
    let mut args = vec!["run".to_string()];
    args.extend(state_args(app)?);
    args.extend(["-owner-pid".into(), owner_pid]);
    let (mut events, child) = app
        .shell()
        .sidecar("ovrenNET")
        .map_err(|_| "OvrenNET is unavailable on this device.".to_string())?
        .args(args)
        .spawn()
        .map_err(|_| "Could not start OvrenNET.".to_string())?;
    tauri::async_runtime::spawn(async move {
        while let Some(event) = events.recv().await {
            if matches!(event, CommandEvent::Terminated(_)) {
                break;
            }
        }
        drop(child);
    });
    Ok(())
}

pub async fn stop(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["off".to_string()];
    args.extend(state_args(app)?);
    let output = app
        .shell()
        .sidecar("ovrenNET")
        .map_err(|_| "OvrenNET is unavailable on this device.".to_string())?
        .args(args)
        .output()
        .await
        .map_err(|_| "Could not stop OvrenNET.".to_string())?;
    if output.status.success() {
        Ok(())
    } else {
        Err("Could not stop OvrenNET.".into())
    }
}
