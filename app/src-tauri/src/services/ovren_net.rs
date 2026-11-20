use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::path::PathBuf;
use tauri::Manager;
use tauri_plugin_shell::{process::CommandEvent, ShellExt};

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct Config {
    pub included_adapters: Vec<String>,
    pub auto_weight: bool,
    pub weights: std::collections::HashMap<String, f64>,
    pub limits_mbps: std::collections::HashMap<String, f64>,
    pub ipv6_mode: String,
    pub auto_pause: bool,
    pub mtu: u16,
    pub start_with_ovren: bool,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            included_adapters: Vec::new(),
            auto_weight: true,
            weights: std::collections::HashMap::new(),
            limits_mbps: std::collections::HashMap::new(),
            ipv6_mode: "auto".into(),
            auto_pause: false,
            mtu: 9000,
            start_with_ovren: false,
        }
    }
}

impl Config {
    pub fn validate(&self) -> Result<(), String> {
        if !matches!(self.ipv6_mode.as_str(), "auto" | "on" | "off") {
            return Err("Invalid OvrenNET IPv6 mode.".into());
        }
        if !(576..=9000).contains(&self.mtu) {
            return Err("OvrenNET MTU must be between 576 and 9000.".into());
        }
        if self.included_adapters.len() > 32
            || self.weights.len() > 32
            || self.limits_mbps.len() > 32
        {
            return Err("Too many OvrenNET adapter settings.".into());
        }
        let mut names = HashSet::new();
        for name in &self.included_adapters {
            if !valid_adapter_name(name) || !names.insert(name.to_lowercase()) {
                return Err("Invalid or duplicate OvrenNET adapter name.".into());
            }
        }
        for (name, weight) in &self.weights {
            if !valid_adapter_name(name) || !weight.is_finite() || !(0.05..=1000.0).contains(weight)
            {
                return Err("Invalid OvrenNET adapter weight.".into());
            }
        }
        for (name, limit) in &self.limits_mbps {
            if !valid_adapter_name(name) || !limit.is_finite() || !(0.0..=100000.0).contains(limit)
            {
                return Err("Invalid OvrenNET adapter speed limit.".into());
            }
        }
        Ok(())
    }
}

fn valid_adapter_name(value: &str) -> bool {
    !value.is_empty()
        && value.chars().count() <= 48
        && value.trim() == value
        && !value.chars().any(char::is_control)
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct AdapterSnapshot {
    pub name: String,
    pub connected: bool,
    pub download_bytes_per_second: f64,
    pub upload_bytes_per_second: f64,
    pub link_download_bytes_per_second: f64,
    pub link_upload_bytes_per_second: f64,
    pub active_flows: i64,
    pub total_flows: i64,
    pub state: String,
    pub ipv6_state: String,
    pub weight: f64,
    pub weight6: f64,
    pub limit_mbps: f64,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Snapshot {
    pub running: bool,
    pub enabled: bool,
    pub paused_reason: String,
    pub since: Option<String>,
    pub adapters: Vec<AdapterSnapshot>,
    pub config: Config,
    pub config_error: String,
    pub socks_running: bool,
}

fn prepare_state_directory(
    app: &tauri::AppHandle,
    suffix: Option<&str>,
) -> Result<PathBuf, String> {
    let mut directory = app
        .path()
        .app_cache_dir()
        .map_err(|_| "Could not prepare OvrenNET storage.".to_string())?
        .join("OvrenNET");
    if let Some(suffix) = suffix {
        directory.push(suffix);
    }
    let control = directory.join("ctl");
    std::fs::create_dir_all(&control)
        .map_err(|_| "Could not prepare OvrenNET storage.".to_string())?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(&directory, std::fs::Permissions::from_mode(0o700))
            .and_then(|_| {
                std::fs::set_permissions(&control, std::fs::Permissions::from_mode(0o700))
            })
            .map_err(|_| "Could not secure OvrenNET storage.".to_string())?;
    }
    Ok(directory)
}

fn state_args(app: &tauri::AppHandle) -> Result<Vec<String>, String> {
    Ok(vec![
        "-state-dir".into(),
        prepare_state_directory(app, None)?
            .to_string_lossy()
            .into_owned(),
    ])
}

fn socks_state_args(app: &tauri::AppHandle) -> Result<Vec<String>, String> {
    let directory = prepare_state_directory(app, Some("socks"))?;
    Ok(vec![
        "-socks-state-dir".into(),
        directory.to_string_lossy().into_owned(),
    ])
}

fn socks_run_state_args(app: &tauri::AppHandle) -> Result<Vec<String>, String> {
    let directory = prepare_state_directory(app, Some("socks"))?;
    Ok(vec![
        "-state-dir".into(),
        directory.to_string_lossy().into_owned(),
    ])
}

fn config_args(config: &Config) -> Result<Vec<String>, String> {
    config.validate()?;
    let value = serde_json::to_string(config)
        .map_err(|_| "Could not prepare OvrenNET settings.".to_string())?;
    Ok(vec!["-config-json".into(), value])
}

fn command_error(output: &tauri_plugin_shell::process::Output, fallback: &str) -> String {
    let detail = String::from_utf8_lossy(&output.stderr)
        .chars()
        .filter(|character| !character.is_control() || *character == '\t')
        .take(512)
        .collect::<String>();
    let detail = detail.trim();
    if detail.is_empty() {
        fallback.into()
    } else {
        format!("{fallback} {detail}")
    }
}

async fn output(app: &tauri::AppHandle, args: Vec<String>, fallback: &str) -> Result<(), String> {
    let result = app
        .shell()
        .sidecar("ovrenNET")
        .map_err(|_| "OvrenNET is unavailable on this device.".to_string())?
        .args(args)
        .output()
        .await
        .map_err(|_| fallback.to_string())?;
    if result.status.success() {
        Ok(())
    } else {
        Err(command_error(&result, fallback))
    }
}

async fn spawn(app: &tauri::AppHandle, args: Vec<String>) -> Result<(), String> {
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

pub async fn snapshot(app: &tauri::AppHandle, config: &Config) -> Result<Snapshot, String> {
    config.validate()?;
    let config_json = serde_json::to_string(config)
        .map_err(|_| "Could not prepare OvrenNET settings.".to_string())?;
    let mut args = vec!["snapshot".to_string()];
    args.extend(state_args(app)?);
    args.extend(socks_state_args(app)?);
    args.extend(["-config-json".into(), config_json]);
    let result = app
        .shell()
        .sidecar("ovrenNET")
        .map_err(|_| "OvrenNET is unavailable on this device.".to_string())?
        .args(args)
        .output()
        .await
        .map_err(|_| "Could not read OvrenNET status.".to_string())?;
    if !result.status.success() {
        return Err(command_error(&result, "Could not read OvrenNET status."));
    }
    serde_json::from_slice(&result.stdout).map_err(|_| "OvrenNET returned invalid status.".into())
}

pub async fn start(app: &tauri::AppHandle, config: &Config) -> Result<(), String> {
    config.validate()?;
    let current = snapshot(app, config).await?;
    if current.running {
        configure(app, config).await?;
        return resume(app).await;
    }
    let mut args = vec!["run".to_string()];
    args.extend(state_args(app)?);
    args.extend(["-owner-pid".into(), std::process::id().to_string()]);
    args.extend(config_args(config)?);
    spawn(app, args).await
}

pub async fn configure(app: &tauri::AppHandle, config: &Config) -> Result<(), String> {
    let mut args = vec!["configure".to_string()];
    args.extend(state_args(app)?);
    args.extend(config_args(config)?);
    output(app, args, "Could not apply OvrenNET settings.").await
}

pub async fn pause(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["pause".to_string()];
    args.extend(state_args(app)?);
    output(app, args, "Could not pause OvrenNET.").await
}

pub async fn resume(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["resume".to_string()];
    args.extend(state_args(app)?);
    output(app, args, "Could not resume OvrenNET.").await
}

pub async fn stop(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["off".to_string()];
    args.extend(state_args(app)?);
    output(app, args, "Could not stop OvrenNET.").await
}

pub async fn recover(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["recover".to_string()];
    args.extend(state_args(app)?);
    output(app, args, "Could not recover network routes.").await
}

pub async fn socks_start(app: &tauri::AppHandle, config: &Config) -> Result<(), String> {
    config.validate()?;
    let current = snapshot(app, config).await?;
    if current.socks_running {
        return Ok(());
    }
    let mut args = vec!["socks".to_string()];
    args.extend(socks_run_state_args(app)?);
    args.extend(["-owner-pid".into(), std::process::id().to_string()]);
    args.extend(config_args(config)?);
    spawn(app, args).await
}

pub async fn socks_stop(app: &tauri::AppHandle) -> Result<(), String> {
    let mut args = vec!["socks-off".to_string()];
    args.extend(socks_run_state_args(app)?);
    output(app, args, "Could not stop the SOCKS5 proxy.").await
}

pub async fn shutdown(app: &tauri::AppHandle) {
    let _ = stop(app).await;
    let _ = socks_stop(app).await;
}
