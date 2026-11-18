use crate::services::ovren_net::{self, Snapshot};

#[tauri::command]
pub async fn ovren_net_snapshot(app: tauri::AppHandle) -> Result<Snapshot, String> {
    ovren_net::snapshot(&app).await
}

#[tauri::command]
pub async fn ovren_net_start(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::start(&app).await
}

#[tauri::command]
pub async fn ovren_net_stop(app: tauri::AppHandle) -> Result<(), String> {
    ovren_net::stop(&app).await
}
