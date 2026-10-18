# billing/invoices Specification

## Purpose
Issue invoices for paid plans and let account owners download them for their own records.

## Requirements
### Requirement: Invoice Issue
The system SHALL issue an invoice for every successful plan payment.

#### Scenario: Payment succeeds
- **WHEN** a plan payment succeeds
- **THEN** the system issues an invoice for the paid amount

### Requirement: Invoice Export
The system SHALL let an account owner download any issued invoice as a PDF file.

#### Scenario: Owner downloads an invoice
- **WHEN** an account owner selects an issued invoice and chooses download
- **THEN** the system returns the invoice as a PDF file
