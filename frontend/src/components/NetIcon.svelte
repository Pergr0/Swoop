<script lang="ts">
  import autoIcon from "../assets/net/auto.svg";
  import wifiIcon from "../assets/net/wifi.svg";
  import ethernetIcon from "../assets/net/ethernet.svg";
  import tunnelIcon from "../assets/net/tunnel.svg";
  import virtualIcon from "../assets/net/virtual.svg";
  import otherIcon from "../assets/net/other.svg";

  export let kind = "";
  export let size = 28;

  const icons: Record<string, string> = {
    auto: autoIcon,
    wifi: wifiIcon,
    ethernet: ethernetIcon,
    tunnel: tunnelIcon,
    virtual: virtualIcon,
    other: otherIcon,
  };
  const labels: Record<string, string> = {
    auto: "Авто",
    wifi: "Wi-Fi",
    ethernet: "Ethernet",
    tunnel: "VPN / туннель",
    virtual: "Виртуальный адаптер",
    other: "Сеть",
  };
  $: src = icons[kind] ?? otherIcon;
  $: label = labels[kind] ?? "Сеть";
</script>

<img
  class="net-icon"
  {src}
  alt=""
  width={size}
  height={size}
  title={label}
  aria-label={label}
/>

<style>
  .net-icon {
    display: block;
    flex-shrink: 0;
    object-fit: contain;
  }
</style>
