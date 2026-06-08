<script lang="ts">
  import autoIcon from "../assets/net/auto.svg";
  import wifiIcon from "../assets/net/wifi.svg";
  import ethernetIcon from "../assets/net/ethernet.svg";
  import tunnelIcon from "../assets/net/tunnel.svg";
  import virtualIcon from "../assets/net/virtual.svg";
  import otherIcon from "../assets/net/other.svg";
  import { t } from "../i18n";

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
  const labelKeys: Record<string, "net.auto" | "net.wifi" | "net.ethernet" | "net.tunnel" | "net.virtual" | "net.other"> = {
    auto: "net.auto",
    wifi: "net.wifi",
    ethernet: "net.ethernet",
    tunnel: "net.tunnel",
    virtual: "net.virtual",
    other: "net.other",
  };
  $: src = icons[kind] ?? otherIcon;
  $: label = t(labelKeys[kind] ?? "net.other");
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
