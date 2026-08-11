// Build script for NGINX dynamic module
// Handles platform-specific linker flags for undefined NGINX symbols

fn main() {
    // On macOS, allow undefined dynamic_lookup for symbols that will be
    // resolved by NGINX at runtime when the module is loaded
    #[cfg(target_os = "macos")]
    {
        println!("cargo:rustc-cdylib-link-arg=-Wl,-undefined,dynamic_lookup");
    }

    // On Linux, allow undefined symbols
    #[cfg(target_os = "linux")]
    {
        println!("cargo:rustc-cdylib-link-arg=-Wl,--unresolved-symbols=ignore-all");
    }
}
