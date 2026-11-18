use crate::services::ovren_net::{self, Config, Snapshot};

#[tauri::command]
pub async fn ovren_net_snapshot(app: tauri::AppHandle, config: Config) -> Result<Snapshot, String> {
    ovren_net::snapshot(&app, &config).await
}

#[tauri::command]
pub async fn ovren_net_start(app: tauri::AppHandle, config: Config) -> Result<(), String> {
    ovren_net::start(&app, &config).await
}

#[tauri::command]
pub async fn ovren_net_configure(app: tauri::AppHandle, config: Config) -> Result<(), String> {
    ovren_net::configure(&app, &config).await
}

#[tauri::command]
pub async fn ovren_net_pause(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::pause(&app).await
}

#[tauri::command]
pub async fn ovren_net_resume(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::resume(&app).await
}

#[tauri::command]
pub async fn ovren_net_stop(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::stop(&app).await
}

#[tauri::command]
pub async fn ovren_net_recover(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::recover(&app).await
}

#[tauri::command]
pub async fn ovren_net_socks_start(app: tauri::AppHandle, config: Config) -> Result<(), String> {
    ovren_net::socks_start(&app, &config).await
}

#[tauri::command]
pub async fn ovren_net_socks_stop(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::socks_stop(&app).await
}
