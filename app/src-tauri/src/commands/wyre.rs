use serde::{Deserialize, Serialize};
use std::time::{Duration, Instant};

const SPEED_TEST_BASE: &str = "https://speed.cloudflare.com";
const SPEED_TEST_TRANSFER_TIMEOUT: Duration = Duration::from_secs(120);
const SPEED_TEST_PROBE_TIMEOUT: Duration = Duration::from_secs(10);
#[cfg(target_os = "windows")]
const WINDOWS_MOUSE_TYPE: &str = r#"Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class WyreMouse { [DllImport("user32.dll", SetLastError=true)] public static extern bool SystemParametersInfo(int action, int param, IntPtr value, int flags); [DllImport("user32.dll", EntryPoint="SystemParametersInfo", SetLastError=true)] public static extern bool SystemParametersInfoArray(int action, int param, int[] value, int flags); }'"#;

#[cfg(target_os = "windows")]
const WINDOWS_MOUSE_EXT_TYPE: &str = r#"Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class WyreMouseExt { [DllImport("user32.dll", EntryPoint="SystemParametersInfo", SetLastError=true)] public static extern bool SystemParametersInfoOutUInt(int action, uint param, ref uint value, uint flags); [DllImport("user32.dll", EntryPoint="SystemParametersInfo", SetLastError=true)] public static extern bool SystemParametersInfoUInt(int action, uint param, IntPtr value, uint flags); [DllImport("user32.dll")] public static extern int GetSystemMetrics(int index); [DllImport("user32.dll")] public static extern uint GetDoubleClickTime(); }'"#;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMeasurement {
    latency_ms: u32,
    jitter_ms: u32,
    download_mbps: f64,
    upload_mbps: f64,
    failed_probes_percent: u8,
    data_mb: u32,
    server_host: String,
    server_location: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MouseSettings {
    supported: bool,
    speed: Option<u8>,
    acceleration: Option<bool>,
    acceleration_profile: Option<String>,
    primary_button: Option<String>,
    scroll_lines: Option<i32>,
    natural_scroll: Option<bool>,
    double_click_ms: Option<u16>,
    reason: Option<String>,
}

#[tauri::command]
pub async fn network_test() -> Result<NetworkMeasurement, String> {
    let client = reqwest::Client::builder()
        .connect_timeout(SPEED_TEST_PROBE_TIMEOUT)
        .timeout(SPEED_TEST_TRANSFER_TIMEOUT)
        .build()
        .map_err(|error| format!("Could not prepare the network check: {error}"))?;
    let mut samples = Vec::new();
    let mut failed = 0u8;
    let mut last_probe_error = String::new();
    let mut server_host = None;
    let mut server_location = None;
    for _ in 0..4 {
        let started = Instant::now();
        match client
            .get(format!("{SPEED_TEST_BASE}/__down?bytes=0"))
            .timeout(SPEED_TEST_PROBE_TIMEOUT)
            .send()
            .await
            .and_then(reqwest::Response::error_for_status)
        {
            Ok(response) => {
                if server_host.is_none() {
                    server_host = response.url().host_str().map(str::to_string);
                }
                if server_location.is_none() {
                    server_location = cloudflare_location(&response);
                }
                match response.bytes().await {
                    Ok(_) => samples.push(started.elapsed().as_secs_f64() * 1000.0),
                    Err(error) => {
                        failed += 1;
                        last_probe_error = error.to_string();
                    }
                }
            }
            Err(error) => {
                failed += 1;
                last_probe_error = error.to_string();
            }
        }
    }
    if samples.is_empty() {
        return Err(format!(
            "Could not reach the speed test server. Check your internet connection. {last_probe_error}"
        ));
    }
    let latency = samples.iter().sum::<f64>() / samples.len() as f64;
    let jitter = samples
        .windows(2)
        .map(|pair| (pair[1] - pair[0]).abs())
        .sum::<f64>()
        / samples.len().saturating_sub(1).max(1) as f64;

    let download_started = Instant::now();
    let download_response = client
        .get(format!("{SPEED_TEST_BASE}/__down?bytes=10000000"))
        .send()
        .await
        .and_then(reqwest::Response::error_for_status)
        .map_err(|error| format!("Download check failed: {error}"))?;
    if server_location.is_none() {
        server_location = cloudflare_location(&download_response);
    }
    let download = download_response
        .bytes()
        .await
        .map_err(|error| format!("Download check failed: {error}"))?;
    let download_mbps = mbps(download.len(), download_started.elapsed());

    let upload_started = Instant::now();
    let upload_response = client
        .post(format!("{SPEED_TEST_BASE}/__up"))
        .body(vec![0u8; 4_000_000])
        .send()
        .await
        .and_then(reqwest::Response::error_for_status)
        .map_err(|error| format!("Upload check failed: {error}"))?;
    if server_location.is_none() {
        server_location = cloudflare_location(&upload_response);
    }
    upload_response
        .bytes()
        .await
        .map_err(|error| format!("Upload check failed: {error}"))?;
    let upload_mbps = mbps(4_000_000, upload_started.elapsed());

    Ok(NetworkMeasurement {
        latency_ms: latency.round() as u32,
        jitter_ms: jitter.round() as u32,
        download_mbps,
        upload_mbps,
        failed_probes_percent: failed * 25,
        data_mb: 14,
        server_host: server_host.unwrap_or_else(|| "speed.cloudflare.com".into()),
        server_location: server_location.unwrap_or_default(),
    })
}

fn cloudflare_location(response: &reqwest::Response) -> Option<String> {
    let ray = response.headers().get("cf-ray")?.to_str().ok()?;
    let location = ray.rsplit_once('-')?.1;
    if (3..=5).contains(&location.len()) && location.bytes().all(|byte| byte.is_ascii_uppercase()) {
        Some(location.to_string())
    } else {
        None
    }
}

fn mbps(bytes: usize, elapsed: Duration) -> f64 {
    let seconds = elapsed.as_secs_f64().max(0.001);
    ((bytes as f64 * 8.0 / seconds / 1_000_000.0) * 10.0).round() / 10.0
}

#[tauri::command]
pub fn mouse_get_settings() -> MouseSettings {
    #[cfg(target_os = "windows")]
    {
        windows_mouse_settings().unwrap_or_else(unavailable)
    }
    #[cfg(target_os = "linux")]
    {
        if !is_gnome() {
            return unavailable("Mouse controls currently support GNOME desktops on Linux.".into());
        }
        gnome_mouse_settings().unwrap_or_else(unavailable)
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    unavailable("Mouse controls are unavailable on this platform.".into())
}

#[tauri::command]
pub fn mouse_apply_settings(
    speed: u8,
    acceleration: bool,
    acceleration_profile: Option<String>,
    primary_button: Option<String>,
    scroll_lines: Option<i32>,
    natural_scroll: Option<bool>,
    double_click_ms: Option<i32>,
) -> Result<MouseSettings, String> {
    if !(1..=20).contains(&speed) {
        return Err("Choose a pointer speed from 1 to 20.".into());
    }
    validate_mouse_options(primary_button.as_deref(), scroll_lines, double_click_ms)?;
    #[cfg(target_os = "windows")]
    {
        if acceleration_profile.as_deref().is_some_and(|profile| {
            !matches!(profile, "0" | "1" | "2") || (profile != "0") != acceleration
        }) {
            return Err("The saved Windows acceleration setting is not supported.".into());
        }
        if natural_scroll.is_some() {
            return Err("Natural scrolling is not supported on Windows.".into());
        }
    }
    #[cfg(target_os = "linux")]
    if acceleration_profile.as_deref().is_some_and(|profile| {
        !matches!(profile, "default" | "adaptive" | "flat") || (profile != "flat") != acceleration
    }) {
        return Err("The saved pointer acceleration profile is not supported.".into());
    }
    #[cfg(target_os = "linux")]
    if scroll_lines.is_some() || double_click_ms.is_some() {
        return Err("Wheel line and double-click settings are not supported on Linux.".into());
    }

    #[cfg(target_os = "windows")]
    {
        apply_windows_mouse_settings(
            speed,
            acceleration,
            acceleration_profile.as_deref(),
            primary_button.as_deref(),
            scroll_lines,
            double_click_ms,
        )
    }
    #[cfg(target_os = "linux")]
    {
        if !is_gnome() {
            return Err("Mouse controls currently support GNOME desktops on Linux.".into());
        }
        apply_gnome_mouse_settings(
            speed,
            acceleration,
            acceleration_profile.as_deref(),
            primary_button.as_deref(),
            natural_scroll,
        )
    }
    #[cfg(not(any(target_os = "windows", target_os = "linux")))]
    {
        let _ = (
            acceleration,
            acceleration_profile,
            primary_button,
            scroll_lines,
            natural_scroll,
            double_click_ms,
        );
        Err("Mouse controls are unavailable on this platform.".into())
    }
}

fn validate_mouse_options(
    primary_button: Option<&str>,
    scroll_lines: Option<i32>,
    double_click_ms: Option<i32>,
) -> Result<(), String> {
    if primary_button.is_some_and(|button| !matches!(button, "left" | "right")) {
        return Err("Choose either the left or right primary mouse button.".into());
    }
    if scroll_lines.is_some_and(|lines| lines != -1 && !(0..=100).contains(&lines)) {
        return Err("Choose from 0 to 100 wheel lines or one page at a time.".into());
    }
    if double_click_ms.is_some_and(|milliseconds| !(0..=5000).contains(&milliseconds)) {
        return Err("Double-click time must be from 0 to 5000 milliseconds.".into());
    }
    Ok(())
}

#[cfg(target_os = "windows")]
fn windows_mouse_settings() -> Result<MouseSettings, String> {
    let script = format!(
        r#"{WINDOWS_MOUSE_TYPE}; {WINDOWS_MOUSE_EXT_TYPE}; $m=Get-ItemProperty 'HKCU:\Control Panel\Mouse'; $lines=[uint32]0; if(-not [WyreMouseExt]::SystemParametersInfoOutUInt(0x0068,0,[ref]$lines,0)) {{ throw 'Windows could not read the wheel scroll setting' }}; $scroll=[long]$lines; if($lines -eq [uint32]::MaxValue) {{ $scroll=-1 }}; @{{supported=$true;speed=[int]$m.MouseSensitivity;acceleration=([int]$m.MouseSpeed -gt 0);accelerationProfile=([string][int]$m.MouseSpeed);primaryButton=$(if([WyreMouseExt]::GetSystemMetrics(23) -ne 0) {{ 'right' }} else {{ 'left' }});scrollLines=$scroll;naturalScroll=$null;doubleClickMs=[WyreMouseExt]::GetDoubleClickTime();reason=$null}} | ConvertTo-Json -Compress"#
    );
    let output = run(
        "powershell.exe",
        &[
            "-NoLogo",
            "-NoProfile",
            "-NonInteractive",
            "-Command",
            &script,
        ],
    )?;
    serde_json::from_str(&output)
        .map_err(|error| format!("Windows returned invalid pointer settings: {error}"))
}

#[cfg(target_os = "windows")]
fn apply_windows_mouse_settings(
    speed: u8,
    acceleration: bool,
    acceleration_profile: Option<&str>,
    primary_button: Option<&str>,
    scroll_lines: Option<i32>,
    double_click_ms: Option<i32>,
) -> Result<MouseSettings, String> {
    let acceleration_value = acceleration_profile
        .and_then(|profile| profile.parse::<u8>().ok())
        .unwrap_or(u8::from(acceleration));
    let primary_button_value = primary_button.unwrap_or("");
    let scroll_lines_value = scroll_lines.map_or_else(String::new, |lines| lines.to_string());
    let double_click_value =
        double_click_ms.map_or_else(String::new, |milliseconds| milliseconds.to_string());
    let script = format!(
        r#"{WINDOWS_MOUSE_TYPE}; {WINDOWS_MOUSE_EXT_TYPE}; $m=Get-ItemProperty 'HKCU:\Control Panel\Mouse'; $oldSpeed=[int]$m.MouseSensitivity; $oldAccel=[int]$m.MouseSpeed; $t1=[int]$m.MouseThreshold1; $t2=[int]$m.MouseThreshold2; $oldSwap=[WyreMouseExt]::GetSystemMetrics(23) -ne 0; $oldLines=[uint32]0; if(-not [WyreMouseExt]::SystemParametersInfoOutUInt(0x0068,0,[ref]$oldLines,0)) {{ throw 'Windows could not read the wheel scroll setting' }}; $oldDouble=[WyreMouseExt]::GetDoubleClickTime(); $okSpeed=[WyreMouse]::SystemParametersInfo(0x0071,0,[IntPtr]{speed},3); $v=[int[]]@($t1,$t2,{acceleration_value}); $okAccel=[WyreMouse]::SystemParametersInfoArray(0x0004,0,$v,3); $okButton=$true; if('{primary_button_value}' -ne '') {{ $swap=('{primary_button_value}' -eq 'right'); $okButton=[WyreMouse]::SystemParametersInfo(0x0021,[int]$swap,[IntPtr]::Zero,3) }}; $okLines=$true; if('{scroll_lines_value}' -ne '') {{ $newLines=if('{scroll_lines_value}' -eq '-1') {{ [uint32]::MaxValue }} else {{ [uint32]'{scroll_lines_value}' }}; $okLines=[WyreMouseExt]::SystemParametersInfoUInt(0x0069,$newLines,[IntPtr]::Zero,3) }}; $okDouble=$true; if('{double_click_value}' -ne '') {{ $okDouble=[WyreMouse]::SystemParametersInfo(0x0020,[int]'{double_click_value}',[IntPtr]::Zero,3) }}; if(-not ($okSpeed -and $okAccel -and $okButton -and $okLines -and $okDouble)) {{ [void][WyreMouse]::SystemParametersInfo(0x0071,0,[IntPtr]$oldSpeed,3); $old=[int[]]@($t1,$t2,$oldAccel); [void][WyreMouse]::SystemParametersInfoArray(0x0004,0,$old,3); [void][WyreMouse]::SystemParametersInfo(0x0021,[int]$oldSwap,[IntPtr]::Zero,3); [void][WyreMouseExt]::SystemParametersInfoUInt(0x0069,$oldLines,[IntPtr]::Zero,3); [void][WyreMouse]::SystemParametersInfo(0x0020,[int]$oldDouble,[IntPtr]::Zero,3); throw 'Windows rejected the pointer settings' }}; $currentLines=[uint32]0; [void][WyreMouseExt]::SystemParametersInfoOutUInt(0x0068,0,[ref]$currentLines,0); $scroll=[long]$currentLines; if($currentLines -eq [uint32]::MaxValue) {{ $scroll=-1 }}; @{{supported=$true;speed={speed};acceleration=$({acceleration_value} -gt 0);accelerationProfile='{acceleration_value}';primaryButton=$(if([WyreMouseExt]::GetSystemMetrics(23) -ne 0) {{ 'right' }} else {{ 'left' }});scrollLines=$scroll;naturalScroll=$null;doubleClickMs=[WyreMouseExt]::GetDoubleClickTime();reason=$null}} | ConvertTo-Json -Compress"#
    );
    let output = run(
        "powershell.exe",
        &[
            "-NoLogo",
            "-NoProfile",
            "-NonInteractive",
            "-Command",
            &script,
        ],
    )?;
    serde_json::from_str(&output)
        .map_err(|error| format!("Windows returned invalid pointer settings: {error}"))
}

#[cfg(target_os = "linux")]
fn is_gnome() -> bool {
    std::env::var("XDG_CURRENT_DESKTOP")
        .is_ok_and(|desktop| desktop.to_ascii_lowercase().contains("gnome"))
}

#[cfg(target_os = "linux")]
fn gnome_mouse_settings() -> Result<MouseSettings, String> {
    let speed = run(
        "gsettings",
        &["get", "org.gnome.desktop.peripherals.mouse", "speed"],
    )?
    .parse::<f64>()
    .map_err(|error| format!("GNOME returned an invalid pointer speed: {error}"))?;
    let profile = run(
        "gsettings",
        &[
            "get",
            "org.gnome.desktop.peripherals.mouse",
            "accel-profile",
        ],
    )?
    .trim_matches('\'')
    .to_string();
    let left_handed = run(
        "gsettings",
        &["get", "org.gnome.desktop.peripherals.mouse", "left-handed"],
    )?
    .parse::<bool>()
    .map_err(|error| format!("GNOME returned an invalid primary button setting: {error}"))?;
    let natural_scroll = run(
        "gsettings",
        &[
            "get",
            "org.gnome.desktop.peripherals.mouse",
            "natural-scroll",
        ],
    )?
    .parse::<bool>()
    .map_err(|error| format!("GNOME returned an invalid scroll direction setting: {error}"))?;
    Ok(MouseSettings {
        supported: true,
        speed: Some(((speed + 1.0) * 10.0).round().clamp(1.0, 20.0) as u8),
        acceleration: Some(profile != "flat"),
        acceleration_profile: Some(profile),
        primary_button: Some(if left_handed { "right" } else { "left" }.into()),
        scroll_lines: None,
        natural_scroll: Some(natural_scroll),
        double_click_ms: None,
        reason: None,
    })
}

#[cfg(target_os = "linux")]
fn apply_gnome_mouse_settings(
    speed: u8,
    acceleration: bool,
    acceleration_profile: Option<&str>,
    primary_button: Option<&str>,
    natural_scroll: Option<bool>,
) -> Result<MouseSettings, String> {
    let previous = gnome_mouse_settings()?;
    let previous_speed = previous
        .speed
        .ok_or("Could not read the current pointer speed.")?;
    let previous_profile = previous
        .acceleration_profile
        .as_deref()
        .ok_or("Could not read the current acceleration profile.")?;
    let previous_primary = previous
        .primary_button
        .as_deref()
        .ok_or("Could not read the current primary mouse button.")?;
    let previous_natural_scroll = previous
        .natural_scroll
        .ok_or("Could not read the current scroll direction.")?;
    let profile = acceleration_profile.unwrap_or(if acceleration { "adaptive" } else { "flat" });
    let mut changes = vec![
        (
            "speed".to_string(),
            format!("{:.2}", (f64::from(speed) / 10.0) - 1.0),
            format!("{:.2}", (f64::from(previous_speed) / 10.0) - 1.0),
        ),
        (
            "accel-profile".to_string(),
            profile.to_string(),
            previous_profile.to_string(),
        ),
    ];
    if let Some(primary_button) = primary_button {
        changes.push((
            "left-handed".into(),
            (primary_button == "right").to_string(),
            (previous_primary == "right").to_string(),
        ));
    }
    if let Some(natural_scroll) = natural_scroll {
        changes.push((
            "natural-scroll".into(),
            natural_scroll.to_string(),
            previous_natural_scroll.to_string(),
        ));
    }
    let mut applied: Vec<(String, String)> = Vec::new();
    for (key, value, old_value) in changes {
        if let Err(error) = run(
            "gsettings",
            &["set", "org.gnome.desktop.peripherals.mouse", &key, &value],
        ) {
            let mut rollback_succeeded = true;
            for (key, old_value) in applied.into_iter().rev() {
                if run(
                    "gsettings",
                    &[
                        "set",
                        "org.gnome.desktop.peripherals.mouse",
                        &key,
                        &old_value,
                    ],
                )
                .is_err()
                {
                    rollback_succeeded = false;
                }
            }
            return Err(if rollback_succeeded {
                error
            } else {
                format!("{error} Settings could not be fully restored; reload the current values.")
            });
        }
        applied.push((key, old_value));
    }
    gnome_mouse_settings()
}

fn unavailable(reason: String) -> MouseSettings {
    MouseSettings {
        supported: false,
        speed: None,
        acceleration: None,
        acceleration_profile: None,
        primary_button: None,
        scroll_lines: None,
        natural_scroll: None,
        double_click_ms: None,
        reason: Some(reason),
    }
}

#[cfg(any(target_os = "windows", target_os = "linux"))]
fn run(program: &str, args: &[&str]) -> Result<String, String> {
    let output = std::process::Command::new(program)
        .args(args)
        .output()
        .map_err(|error| format!("Could not run {program}: {error}"))?;
    if !output.status.success() {
        let detail = String::from_utf8_lossy(&output.stderr).trim().to_string();
        return Err(if detail.is_empty() {
            format!("{program} failed with status {}.", output.status)
        } else {
            format!("{program} failed: {detail}")
        });
    }
    Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
}

#[cfg(test)]
mod tests {
    use super::validate_mouse_options;

    #[test]
    fn mouse_options_validate_supported_values_and_bounds() {
        assert!(validate_mouse_options(Some("right"), Some(-1), Some(5000)).is_ok());
        assert!(validate_mouse_options(Some("center"), None, None).is_err());
        assert!(validate_mouse_options(None, Some(101), None).is_err());
        assert!(validate_mouse_options(None, None, Some(5001)).is_err());
        assert!(validate_mouse_options(None, None, Some(-1)).is_err());
    }
}
