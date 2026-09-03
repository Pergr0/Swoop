# S020 — mdns

Status: **Planned**

## Purpose

Add mDNS/DNS-SD discovery alongside multicast for networks where DNS-SD is the
preferred local discovery mechanism.

## Requirements

### Requirement: DNS-SD advertisement
The system SHALL advertise and browse Swoop services via mDNS/DNS-SD without
removing multicast discovery.

#### Scenario: DNS-SD only network policy
- **WHEN** multicast group joins are restricted but mDNS works
- **THEN** peers SHALL still be discoverable via DNS-SD
