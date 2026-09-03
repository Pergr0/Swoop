# S022 — windows-firewall

Status: **Planned**

## Purpose

On first Windows run, help the user allow Swoop through the firewall so
discovery and transfers work without obscure failures.

## Requirements

### Requirement: First-run firewall guidance
On Windows first launch, the system SHALL prompt or install a firewall rule
covering the control and data ports Swoop uses.

#### Scenario: Fresh Windows install
- **WHEN** Swoop runs the first time on Windows without an existing rule
- **THEN** the user SHALL be guided to allow inbound connections required for peer transfers
