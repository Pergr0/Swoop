<script lang="ts">
  import windowsIcon from "../assets/os/windows.svg";
  import macosIcon from "../assets/os/macos.svg";
  import linuxIcon from "../assets/os/linux.svg";
  import androidIcon from "../assets/os/android.svg";
  import iosIcon from "../assets/os/ios.svg";
  import desktopIcon from "../assets/os/desktop.svg";

  export let platform = "";
  export let size = 40;

  const icons: Record<string, string> = {
    windows: windowsIcon,
    darwin: macosIcon,
    linux: linuxIcon,
    android: androidIcon,
    ios: iosIcon,
  };
  const labels: Record<string, string> = {
    windows: "Windows",
    darwin: "macOS",
    linux: "Linux",
    android: "Android",
    ios: "iOS",
  };
  $: src = icons[platform] ?? desktopIcon;
  $: label = labels[platform] ?? "Устройство";
</script>

<img
  class="platform-icon"
  {src}
  alt=""
  width={size}
  height={size}
  title={label}
  aria-label={label}
/>

<style>
  .platform-icon {
    display: block;
    flex-shrink: 0;
    object-fit: contain;
  }
</style>
