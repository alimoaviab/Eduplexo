<div align="center">

# 🏫 Eduplexo — Enterprise Education Operating System
### *The Next-Generation Multi-Tenant SaaS Platform for Modern Schools & Institutions*

<br/>

[![Go Backend](https://img.shields.io/badge/Backend-Go_1.22_%2B_Chi-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/Database-PostgreSQL_16-336791?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![React 19](https://img.shields.io/badge/Frontend-React_19_%2B_Vite_6-61DAFB?style=for-the-badge&logo=react&logoColor=black)](https://react.dev)
[![Redis 7](https://img.shields.io/badge/Cache-Redis_7_Alpine-DC382D?style=for-the-badge&logo=redis&logoColor=white)](https://redis.io)
[![Multi-Tenant](https://img.shields.io/badge/Architecture-Isolated_Multi--Tenant-6366F1?style=for-the-badge)](#-multi-tenant-data-isolation)
[![Security](https://img.shields.io/badge/Security-Strict_RBAC_%2B_Bcrypt-10B981?style=for-the-badge)](#-security-governance--data-protection)

<br/>

> **Eduplexo** is a comprehensive, institutional-grade School ERP and Cloud SaaS platform engineered to manage academic operations, student lifecycles, automated fee collections, examination engines, teacher management, and multi-campus coordination under a single unified architecture.

</div>

---

## 📑 Table of Contents

- [Executive Overview](#-executive-overview)
- [The Eduplexo Ecosystem](#-the-eduplexo-ecosystem)
- [Core Platform Capabilities](#-core-platform-capabilities)
  - [1. Academic & Curriculum Lifecycle](#1-academic--curriculum-lifecycle)
  - [2. Student Information System (SIS)](#2-student-information-system-sis)
  - [3. Teacher & Staff Faculty Management](#3-teacher--staff-faculty-management)
  - [4. Financial Billing, Invoicing & Fee Ledgers](#4-financial-billing-invoicing--fee-ledgers)
  - [5. Examination Engine & Automated Report Cards](#5-examination-engine--automated-report-cards)
  - [6. Real-Time Attendance, Leave & Discipline](#6-real-time-attendance-leave--discipline)
  - [7. Virtual Live Classes & Timetables](#7-virtual-live-classes--timetables)
  - [8. Partner, Publisher & Affiliate Network](#8-partner-publisher--affiliate-network)
  - [9. Super Admin Platform Command Center](#9-super-admin-platform-command-center)
- [System Architecture & Engineering Design](#-system-architecture--engineering-design)
- [Multi-Tenant Data Isolation](#-multi-tenant-data-isolation)
- [Security, Governance & Compliance](#-security-governance--data-protection)
- [Institutional Scalability & Performance Benchmarks](#-institutional-scalability--performance-benchmarks)

---

## 🌟 Executive Overview

Modern educational institutions require software that is fast, tamper-proof, accessible across devices, and capable of eliminating administrative overhead. **Eduplexo** replaces fragmented spreadsheets, paper registers, disconnected accounting software, and manual examination calculations with a synchronized, real-time operating system.

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                                EDUPLEXO CLOUD PLATFORM                           │
├───────────────────┬───────────────────┬───────────────────┬──────────────────────┤
│  🏢 School Admin   │  👨‍🏫 Faculty Portal │  🎒 Student/Parent │  🌐 Super Admin      │
│  Operations & ERP │  Grades & Lessons │  Vouchers & Scores│  Platform Governance │
├───────────────────┴───────────────────┴───────────────────┴──────────────────────┤
│                          CORE MULTI-TENANT BACKEND ENGINE                        │
│         High-Throughput Go Micro-Kernel • In-Memory Lookup Indexing              │
├──────────────────────────────────────────────────────────────────────────────────┤
│                           DATA & STATE DURABILITY LAYER                          │
│        PostgreSQL 16 Relational Engine • Redis 7 Distributed Cache              │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### Key Pillars of Eduplexo:
1. **Zero-Latency Interactions**: Fast, reactive SPAs paired with an optimized Go API designed to serve requests in sub-millisecond durations.
2. **Absolute Data Integrity**: Fully relational schema enforced by foreign keys, cascade safety, atomic transactions, and strict schema-level check constraints.
3. **Multi-Stakeholder Collaboration**: Unified access for School Owners, Principals, Accountants, Teachers, Students, Parents, and Commercial Growth Partners.
4. **Autonomous Financial Collection**: Real-time generation of student fee vouchers, partial payment tracking, and balance ledgers with FIFO reconciliation.
5. **Academic Continuity**: Support for historical, active, and upcoming academic sessions with instant single-click session switching.

---

## 🌐 The Eduplexo Ecosystem

Eduplexo delivers dedicated, tailor-made web interfaces tailored specifically for each persona across the institution:

| Portal | Primary Audience | Core Functionality |
| :--- | :--- | :--- |
| **School Workspace** (`app.eduplexo.com`) | School Principals, Administrators, Registrars | Complete operational control, admissions, class scheduling, teacher allocation, fee collection, and platform analytics. |
| **Faculty Portal** (`app.eduplexo.com`) | Class Teachers, Subject Teachers | Attendance marking, marksheet entry, homework assignments, student discipline tracking, and leave approvals. |
| **Student & Parent Portal** (`app.eduplexo.com`) | Enrolled Students, Guardians | Fee voucher downloads, exam performance, digital report cards, timetable tracking, and homework submissions. |
| **Super Admin Panel** (`admin.eduplexo.com`) | Platform Founders, Platform Operations | Global tenant oversight, school approvals, custom plan provisioning, subscription revenue, and platform credentials. |
| **Partner & Publisher Network** (`partner.eduplexo.com`) | Affiliates, Publishers, Regional Sales | Referral attribution, transparent conversion tracking, referral link generators, and commission reporting. |

---

## 🧩 Core Platform Capabilities

### 1. Academic & Curriculum Lifecycle
* **Multi-Year Session Management**: Seamlessly transition between past, active, and upcoming academic years without locking or duplicating historic archives.
* **Curriculum Hierarchy**: Class, Section, and Subject configuration with flexible grading weightage, subject credit hours, and electives.
* **Class Teacher & Faculty Assignment**: Assign primary mentors to specific class sections while mapping specialist subject teachers across grades.

### 2. Student Information System (SIS)
* **Digital Student Dossier**: Track student admissions, enrollment IDs, personal biometrics, emergency guardian contacts, and blood groups.
* **Roll Number & Admission Automation**: Automatic generation of institutional identification codes and roll numbers.
* **Parent-Child Association**: Connect parents to one or multiple siblings under a unified family view.

### 3. Teacher & Staff Faculty Management
* **Faculty Profiles**: Record qualifications, designations (e.g., M.Sc Mathematics, Head of Science), employment status, and contact records.
* **Workload & Subject Allocation**: Real-time visibility into teacher timetable allocations, preventing double-booking of teachers across classrooms.

### 4. Financial Billing, Invoicing & Fee Ledgers
* **Dynamic Fee Structures**: Configure tuition fees, admission charges, science lab funds, transportation, and examination charges on a per-class basis.
* **Voucher Generation**: Single-click bulk monthly fee generation producing structured, verifiable student fee invoices.
* **Automated FIFO Reconciliation**: Payments automatically settle the oldest outstanding invoices first, supporting partial payments, discounts, and late-fee penalties.
* **Receipt & Invoice Printing**: Professional, printable PDF and web receipts with official institutional branding and transaction numbers.

### 5. Examination Engine & Automated Report Cards
* **Flexible Exam Architectures**: Manage Mid-Terms, Term Finals, Monthly Class Quizzes, and Diagnostic Assessments.
* **Grade Calculations**: Automated grading systems (A+, A, B, C, F) with customizable threshold policies, percentages, and total marks summaries.
* **Printable Marksheets**: Generate student report cards complete with position in class, teacher remarks, and subject-wise breakdown.

### 6. Real-Time Attendance, Leave & Discipline
* **Batch Daily Attendance**: Fast, intuitive interface for faculty to mark entire classes as Present, Absent, Late, or Excused in seconds.
* **Leave Management**: Student and staff leave requests with administrative review workflows (`pending`, `approved`, `rejected`).
* **Behavioral Merit & Demerit Tracking**: Incident logging by teachers to document achievements, conduct infractions, and awards.

### 7. Virtual Live Classes & Timetables
* **Weekly Schedule Grid**: Visual Monday-through-Friday timetables mapping classroom periods, teacher allocations, and lab locations.
* **Interactive Live Classes**: Integrated online classroom sessions (Zoom / Google Meet) scheduled directly within the academic calendar.

### 8. Partner, Publisher & Affiliate Network
* **Referral Attribution Engine**: Transparent attribution tracking connecting newly onboarded schools to registered regional publishers.
* **Custom Partner Links**: Dedicated referral code generation with zero-friction onboarding for new school registrations.

### 9. Super Admin Platform Command Center
* **Tenant Lifecycle Control**: Instant activation, suspension, review, and deletion of school accounts.
* **Subscription Management**: Free Trial rules, Pro plans, custom student capacity limits, and manual renewals.
* **Platform Security**: Self-service Super Admin credential management, direct password updating, and security session controls.

---

## 🏗️ System Architecture & Engineering Design

Eduplexo is architected around clean, decoupled domain-driven design, maximizing uptime and compute efficiency:

```mermaid
flowchart TB
    subgraph Client Tier
        SA["Super Admin App\n(admin.eduplexo.com)"]
        SW["School & Student App\n(app.eduplexo.com)"]
        PA["Partner Portal\n(partner.eduplexo.com)"]
    end

    subgraph Gateway & Proxy
        NGINX["Nginx Edge Proxy\nTLS Termination • Gzip • HTTP/2"]
    end

    subgraph Backend Core
        GO["Go Backend Service (Chi Router)\nIn-Memory Store • O(1) Indexing • Domain Handlers"]
    end

    subgraph Data & Storage
        PG[("PostgreSQL 16\nACID Relational Engine\nCascading Constraints")]
        REDIS[("Redis 7\nSession Revocation\nJob Queues")]
        UPLOADS["Encrypted Media\nSchool Logos & Attachments"]
    end

    Client Tier --> NGINX
    NGINX --> GO
    GO <--> PG
    GO <--> REDIS
    GO <--> UPLOADS
```

### Architectural Highlights:
* **The Go Engine**: Built with native Go concurrency patterns, utilizing the Chi router for lightweight, zero-allocation request routing.
* **In-Memory Cache with Asynchronous Persistence**: Critical lookup paths (tenants, users, session state) run through an in-memory `MemStore` with thread-safe read/write mutexes, backed by write-through persistence to PostgreSQL.
* **Zero Relational Debt**: The database utilizes strict relational foreign keys with `ON DELETE CASCADE` rules, composite indexes on `(school_id, active_academic_year_id)`, and `citext` for case-insensitive indexing.

---

## 🛡️ Security, Governance & Data Protection

Security in Eduplexo is enforced across every layer of the network, database, and presentation tiers:

```
┌────────────────────────────────────────────────────────────────────────┐
│                        SECURITY GUARANTEES                             │
├──────────────────────────┬─────────────────────────────────────────────┤
│ 🔒 Authentication        │ JWT + Session Revocation in Redis           │
│ 🛡️ Password Encryption   │ Adaptive Bcrypt Hashing (Cost Factor 10)     │
│ 🏢 Tenant Isolation      │ Strict `school_id` Scoping on Every Query   │
│ 👮 Role Authorization    │ Granular RBAC (Admin, Teacher, Student)     │
│ ⚡ Rate Limiting         │ IP & Account Throttling on Public Auth APIs │
│ 🌐 Transport Security    │ Enforced HTTPS, HSTS & Secure Cookies       │
└──────────────────────────┴─────────────────────────────────────────────┘
```

* **Tenant Isolation**: Every database interaction is scoped to the authenticated caller's `school_id`. Cross-tenant requests are intercepted and rejected at the middleware tier.
* **Least-Privilege RBAC**: Faculty and students only have read and write permissions to records associated with their designated classes or personal enrollments.
* **Password Verification**: Credentials are cryptographically hashed using bcrypt. Passwords are never stored in plaintext and never leaked in API envelopes.

---

## 📊 Institutional Scalability & Performance Benchmarks

* **High Concurrency**: Capable of handling thousands of simultaneous student grade inquiries, daily attendance roll calls, and invoice lookups with low memory overhead.
* **Database Efficiency**: Over 90 specialized B-tree indexes guarantee fast queries even in institutions with tens of thousands of student records.
* **Lightweight Footprint**: The compiled Go backend runs in a minimal, secure Docker container with minimal memory overhead, leaving compute resources free for database transactions and caching.

---

<div align="center">

**Eduplexo** — *Empowering Institutions. Elevating Education.*

© 2026 Eduplexo Platform Inc. All rights reserved.

</div>
