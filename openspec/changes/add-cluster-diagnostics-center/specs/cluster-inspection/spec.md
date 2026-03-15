## ADDED Requirements

### Requirement: The system SHALL support on-demand cluster inspections

The system SHALL allow operators to manually start a diagnostic inspection for a managed Seatunnel cluster from the diagnostics workspace and from the cluster detail page. Each inspection MUST create a persisted inspection report with execution status, start/end time, cluster scope, and operator metadata.

#### Scenario: Start inspection from diagnostics workspace

- **WHEN** a user starts a cluster inspection from the diagnostics workspace
- **THEN** the system SHALL create a new inspection execution and return a report identifier that can be used to observe progress and results

#### Scenario: Start inspection from cluster detail

- **WHEN** a user starts an inspection from a cluster detail page
- **THEN** the system SHALL create the inspection with that cluster preselected and link the result back to the same cluster

#### Scenario: Start inspection with a custom recent window

- **WHEN** a user manually starts an inspection and specifies a custom recent window such as 15, 30, or 120 minutes
- **THEN** the system SHALL persist that lookback window on the inspection report
- **AND** recent error-burst and process-event findings SHALL only evaluate signals within that requested window

#### Scenario: Persist failed inspection execution for later review

- **WHEN** an inspection report is created successfully but one of the managed signal sources fails during evaluation
- **THEN** the system SHALL keep the inspection report, mark it as `failed`, and persist the failure reason for later review

### Requirement: Inspections SHALL evaluate managed runtime and health signals

The system SHALL evaluate inspection findings using already managed runtime and health data. The first batch of inspection checks MUST at least cover node online status, process lifecycle events, alert signals, and recent Seatunnel error groups, and MUST remain extensible for later monitor configuration and metrics reachability checks. Inspection checks MUST produce structured findings with severity, summary, and recommendation.

#### Scenario: Detect unhealthy node during inspection

- **WHEN** an inspected cluster contains a node that remains offline or has recent restart-failed events
- **THEN** the inspection report SHALL contain a finding describing the unhealthy node and a recommended next action

#### Scenario: Detect recent high-frequency errors during inspection

- **WHEN** the inspected cluster has recent error groups above the configured inspection threshold
- **THEN** the inspection report SHALL include a finding that links to the related error groups

#### Scenario: Ignore only-historical error totals outside the recent window

- **WHEN** an error group has a high historical total count but fewer than the inspection threshold occurrences inside the recent inspection window
- **THEN** the inspection report SHALL NOT raise a recent-error-burst finding only because of the historical aggregate count

#### Scenario: Detect recent error burst across mixed timezone timestamps

- **WHEN** recent Seatunnel error events are stored in UTC timestamps and the inspection is executed in a non-UTC local timezone
- **THEN** the inspection report SHALL still evaluate the recent error window correctly and include the related error-group finding

### Requirement: The system SHALL present inspection reports and findings for triage

The system SHALL provide list and detail views for inspection reports. Report detail MUST show the execution summary, findings grouped by severity, related nodes, related errors, and follow-up actions including links to error center and diagnostic bundle creation.

#### Scenario: Browse inspection history

- **WHEN** a user opens the inspection tab in diagnostics workspace
- **THEN** the system SHALL show inspection reports with filters for cluster, status, severity, and time range

#### Scenario: Filter inspection history by finding severity

- **WHEN** a user filters inspection history by a severity level such as `warning` or `critical`
- **THEN** the system SHALL only return reports that contain at least one finding of the requested severity

#### Scenario: View inspection finding detail

- **WHEN** a user opens an inspection report detail view
- **THEN** the system SHALL show each finding with its evidence summary and available follow-up actions

#### Scenario: Jump from inspection finding to related diagnostics entry

- **WHEN** an inspection finding references a related error group or requires follow-up evidence collection
- **THEN** the detail view SHALL provide follow-up actions that jump to the related error center context and to the diagnostic-task entry with the current finding context preserved
