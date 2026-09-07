[PERSONA]

You are a senior Staff Full-Stack Engineer, Backend Architect, Database Engineer, Subscription/Billing Architect, Security Engineer, React + TypeScript Engineer, and QA Engineer working inside the EXISTING EduPlexo monorepo.

You are extending a real existing application.

DO NOT build a greenfield system.
DO NOT rewrite unrelated functionality.
DO NOT break existing school modules.
DO NOT blindly create duplicate pricing, subscription, authentication, or referral systems.

Before changing anything, inspect the existing implementation in depth.

You must understand:

- monorepo structure
- existing React applications
- existing Admin application
- existing school application
- existing Publisher/Referral implementation
- authentication
- authorization
- school signup
- school creation
- school admin creation
- pricing/plans
- custom plans
- subscription creation
- trial logic
- module/package assignment
- school subscription page
- database schema
- API layer
- migrations
- Docker
- Vercel deployment
- existing UI components


==================================================
CURRENT ARCHITECTURE
==================================================

EduPlexo currently has:

Admin portal:
https://admin.eduplexo.com

School application:
https://app.eduplexo.com

Publisher portal:
https://refer.eduplexo.com

The legacy Owner architecture has been removed.

The active school roles are:

Admin
Teacher
Student

The School Admin is the administrator for ONE school.

There is no active multi-campus Owner model.


==================================================
CURRENT SIGNUP / PRICING CONTEXT
==================================================

The existing school Create Account flow currently contains pricing choices/cards.

There are multiple plan cards, including a Custom plan and other pricing options.

A user can currently select a pricing plan during account creation.

The existing school Subscription page also displays subscription information such as:

- active plan
- student capacity
- enabled modules/features
- trial status
- plan information

The existing subscription page currently has behavior similar to:

Free Trial
Student Capacity
Included Modules & Features

This existing subscription system must continue working.

We are NOT replacing the entire subscription system.

We are extending it so that a publisher referral can carry a specific subscription offer.


==================================================
NEW BUSINESS REQUIREMENT
==================================================

A Publisher referral link must be tied to a specific subscription offer.

When Admin generates a referral token for a Publisher, Admin must choose the plan/offer that the referred school will receive.

Example:

Publisher:
ABC Education

Referral Token:
secure-random-token

Selected Plan:
Professional

Monthly Price:
Rs. 12,000 / month


OR:

Selected Plan:
Custom

Monthly Price:
Rs. 18,500 / month


The generated referral link is therefore not simply:

"this school belongs to Publisher A"

It also means:

"this school is being onboarded through Publisher A using THIS specific subscription offer."


==================================================
MOST IMPORTANT RULE
==================================================

The referral token must store/lock the referral offer.

Conceptually:

Referral Token
    ↓
Publisher
    ↓
Plan
    ↓
Monthly Price Snapshot
    ↓
Referral School


Do NOT rely only on the current live price from the plan table at the time the school signs up.

Example:

Admin creates token today:

Plan:
Professional

Price:
Rs. 12,000/month


Later Admin changes Professional to:

Rs. 15,000/month


The old referral token must still represent:

Rs. 12,000/month

unless Admin explicitly revokes/recreates it.

This means the referral offer must preserve a price snapshot.


==================================================
REFERRAL TOKEN MODEL
==================================================

Use the existing referral token implementation if present.

Extend it where necessary.

Conceptually the token should have:

- id
- publisher_id
- token_hash
- status
- plan_id
- plan_name_snapshot
- monthly_price_snapshot
- currency
- billing_period
- created_at
- expires_at
- used_at
- used_by_school_id
- revoked_at

Exact field names must match the current schema conventions.

Do not blindly create duplicate models.

If an existing referral-token model already exists, extend it safely.


==================================================
ONE TOKEN = ONE REFERRAL
==================================================

Each generated token is unique and one-time-use.

Example:

Publisher A

Token #001
Plan = Professional
Price = Rs. 12,000
Status = UNUSED

Token #002
Plan = Custom
Price = Rs. 18,500
Status = UNUSED


Using Token #001:

School A
    ↓
Professional
    ↓
Rs. 12,000/month


Token #001 then becomes:

USED


It cannot be used again.


==================================================
ADMIN FLOW — CREATE PUBLISHER
==================================================

Inside the existing Admin portal:

Publishers

Admin can:

Create Publisher
View Publisher
View Details


Publisher has a permanent login.

Publisher login is separate from referral tokens.


==================================================
ADMIN FLOW — PUBLISHER DETAIL
==================================================

Admin opens:

Publishers
    ↓
ABC Education
    ↓
View Detail


Publisher Detail should show useful information.

Example:

ABC Education

Status:
Active

Login:
Active

Referrals:
8

Schools:
8

Then:

Referral Links

[ Generate Referral Link ]


==================================================
NEW — GENERATE REFERRAL LINK FLOW
==================================================

When Admin clicks:

Generate Referral Link

DO NOT immediately generate a token.

First show a proper creation modal/form.


Example:

Create Referral Link

Publisher:
ABC Education

Select Plan:
[ Professional ▼ ]

Monthly Price:
[ Rs. 12,000 ]

Currency:
[ PKR ]

Billing Period:
[ Monthly ]

Referral Expiry:
[ Optional ]

[ Generate Link ]


==================================================
PLAN SELECTION
==================================================

The plan selector must use the EXISTING EduPlexo plan/custom-plan system.

Do not create duplicate plan definitions.

If the existing system has:

- Free
- Basic
- Professional
- Premium
- Custom

reuse those records.

The Admin should select the plan that the referred school will receive.

If the selected plan has a normal configured monthly price, show that price automatically.

Example:

Professional
Configured Price:
Rs. 15,000/month


Admin may have permission to set a referral-specific price.

Example:

Plan:
Professional

Default Price:
Rs. 15,000

Referral Price:
Rs. 12,000


The referral token should save:

plan_id = Professional
monthly_price_snapshot = Rs. 12,000


This allows the business to give a publisher a special negotiated price.


==================================================
CUSTOM PLAN SUPPORT
==================================================

If "Custom" is an existing supported plan type, allow it.

Example:

Plan:
Custom

Monthly Price:
Rs. 18,500


The referral token must preserve the exact selected pricing information.

Do not rely on the Custom plan's future configuration changing the old referral.

Snapshot the agreed referral pricing.


==================================================
REFERRAL LINK CREATION
==================================================

After Admin submits:

Generate Link

Backend must:

1. Validate Publisher.
2. Validate selected Plan.
3. Validate price.
4. Create secure random token.
5. Associate token with Publisher.
6. Associate token with selected Plan.
7. Snapshot monthly price.
8. Set status = UNUSED.
9. Persist metadata.
10. Return raw referral link ONCE.


Example generated link:

https://app.eduplexo.com/invite/8fK92LmP...


==================================================
ONE-TIME SECRET MODAL
==================================================

After successful generation, show:

-----------------------------------------

Referral Link Created

Publisher:
ABC Education

Plan:
Professional

Monthly Price:
Rs. 12,000 / month

IMPORTANT:
This referral link will only be shown once.

Save it somewhere secure.

[ Copy Link ]

[ Done ]

-----------------------------------------

After Admin clicks Done:

The raw link/token MUST NOT be shown again.

The system may later display metadata:

Token #001
Professional
Rs. 12,000/month
UNUSED


but NEVER reveal the raw token again.

If lost:

Admin must generate a NEW referral token.


==================================================
REFERRAL LINK
==================================================

The link should open:

https://app.eduplexo.com/invite/{token}


The existing school application must recognize that this is a referral-driven onboarding flow.


==================================================
CRITICAL SCHOOL SIGNUP BEHAVIOR
==================================================

When a school comes through a valid referral token:

DO NOT show the normal public pricing selection.

This is a critical requirement.

Normal signup:

app.eduplexo.com
    ↓
Create Account
    ↓
Pricing Cards
    ↓
User selects plan


Referral signup:

app.eduplexo.com/invite/{token}
    ↓
Validate token
    ↓
Identify referral offer
    ↓
DO NOT show public pricing cards
    ↓
Show ONLY assigned referral plan
    ↓
Create School
    ↓
Create School Admin


The school must not be able to switch to another plan during this referral onboarding flow unless an explicit business rule allows it.


==================================================
REFERRAL LANDING / SIGNUP UI
==================================================

When school opens the referral link, show a clean page indicating the assigned offer.

Example:

Welcome to EduPlexo

You have been invited to join EduPlexo.

Your Plan

Professional

Rs. 12,000 / month

Included Features
- ...
- ...
- ...

[ Continue Registration ]


Do NOT show:

Basic
Professional
Premium
Custom
other public pricing cards

when the signup originated from a referral token.


==================================================
PLAN LOCK
==================================================

The selected referral plan must be LOCKED.

The frontend must not be able to change:

plan_id
monthly_price
publisher_id
referral_token


The backend must derive all of these from the validated referral token.

DO NOT trust:

plan_id sent by React
price sent by React
publisher_id sent by React


The backend must resolve:

token
    ↓
publisher
    ↓
plan
    ↓
price snapshot


==================================================
SCHOOL CREATION
==================================================

When signup is successfully completed through a referral:

Create:

1. School
2. School Admin
3. Subscription
4. Publisher-School assignment
5. Referral record
6. Referral subscription pricing snapshot
7. Token usage


NEVER create:

Owner

NEVER create:

Owner + Campus


The correct relationship is:

Publisher
    ↓
Referral Token
    ↓
School
    ↓
School Admin
    ↓
Subscription


==================================================
SUBSCRIPTION CREATION
==================================================

When the referral signup completes, create the school subscription using the referral offer.

Example:

Referral:

Publisher:
ABC Education

Plan:
Professional

Monthly Price:
Rs. 12,000


School subscription:

Plan:
Professional

Monthly Price:
Rs. 12,000

Billing Period:
Monthly


The subscription must reflect the referral-specific pricing snapshot.


==================================================
EXISTING SCHOOL SUBSCRIPTION PAGE
==================================================

The existing school Subscription page must continue to work.

However, when the school was created through a referral offer, the subscription page should show the assigned plan.

Example:

Your School Plan

Professional

Rs. 12,000 / month

Student Capacity:
500

Included Modules:
...

The existing subscription information should use real subscription data.


==================================================
IMPORTANT — HIDE OTHER PRICING CARDS
==================================================

For a referral-created school, the normal plan-selection cards must NOT appear as competing options on the relevant subscription/onboarding experience.

The UI should communicate:

Your Current Plan

Professional

Rs. 12,000/month


Instead of showing:

Free
Basic
Professional
Premium
Custom


The user should not become confused and think they can select another plan during referral onboarding.


==================================================
TRIAL LOGIC
==================================================

Inspect the existing EduPlexo trial/subscription behavior.

Do not blindly remove Free Trial behavior.

Determine whether a referral offer should:

- immediately activate the selected plan
OR
- start a trial using the selected plan


This must follow the existing subscription architecture and business rules.

If no referral-specific trial rule exists, preserve the current subscription activation behavior while ensuring the selected referral plan/price is locked.

Do not create fake trial logic.


==================================================
MODULES / FEATURES
==================================================

The selected plan's included modules/features must continue to be applied using the EXISTING plan/package/module architecture.

Example:

Professional
    ↓
Student Directory
Teacher Management
Classes
Attendance
Homework
Exams
Fees
etc.


Do NOT hardcode module lists inside the referral frontend.

Use the existing plan/module configuration system.

The referral token should reference the appropriate plan and preserve the referral pricing snapshot.


==================================================
PUBLISHER DASHBOARD
==================================================

Publisher logs into:

https://refer.eduplexo.com


Publisher dashboard should show:

Welcome back, ABC Education

Assigned Schools
8

Successful Referrals
8

Pending Referrals
0


Then:

Recent Referrals

School
Plan
Monthly Value
Status

Oak Academy
Professional
Rs. 12,000/month
Active

Summit School
Custom
Rs. 18,500/month
Active


Use real data only.


==================================================
PUBLISHER REFERRAL PAGE
==================================================

Show the publisher's own referral history.

Example:

Referral #001

School:
Oak Academy

Plan:
Professional

Monthly Price:
Rs. 12,000

Status:
Successful

Referral #002

School:
Summit School

Plan:
Custom

Monthly Price:
Rs. 18,500

Status:
Successful


Publisher must only see its own records.


==================================================
COMMISSION
==================================================

Do NOT invent a commission calculation formula.

The referral should preserve enough information for commission calculation.

The referral record should contain:

publisher_id
referral_token_id
school_id
plan_id
monthly_price_snapshot
commission_status
commission_amount where defined
created_at


If the existing EduPlexo system already has a commission rule, apply it using the referral's captured pricing.

If no commission rule exists yet:

- do NOT invent one
- store the referral value
- show commission status as Pending/Not configured where appropriate


Commission must not be generated simply because a token was created.


==================================================
ADMIN REFERRAL DETAILS
==================================================

Inside:

Admin
→ Publishers
→ ABC Education
→ Referral Links


Show:

Token #001
Plan:
Professional

Referral Price:
Rs. 12,000/month

Status:
USED

School:
Oak Academy

Created:
Sep 7, 2026

Used:
Sep 8, 2026


Token #002
Plan:
Custom

Referral Price:
Rs. 18,500/month

Status:
UNUSED


Raw token must not be shown.


==================================================
REFERRAL TOKEN IMMUTABILITY
==================================================

After a token has been generated:

The following values must be treated as immutable for that token:

publisher_id
plan_id
plan_name_snapshot
monthly_price_snapshot
currency
billing_period


If Admin wants a different plan or price:

Generate a NEW referral token.

Do not mutate the existing unused token in a way that makes an already-distributed link silently change.


==================================================
TOKEN SECURITY
==================================================

Token must be:

- cryptographically random
- non-predictable
- one-time-use
- revocable
- optionally expirable
- server-validated


Prefer storing only a secure hash of the raw token.

Do not expose raw tokens in later APIs.


==================================================
BACKEND AUTHORIZATION
==================================================

Never trust client-provided:

publisher_id
school_id
plan_id
price
commission
referral ownership


The backend must derive referral information from the authenticated session and validated token.


==================================================
TRANSACTION SAFETY
==================================================

Referral completion must be atomic where supported.

Transaction should cover:

validate token
    ↓
confirm UNUSED
    ↓
resolve publisher
    ↓
resolve plan
    ↓
resolve price snapshot
    ↓
create school
    ↓
create school admin
    ↓
create subscription
    ↓
attach school to publisher
    ↓
create referral
    ↓
mark token USED


If anything fails:

ROLL BACK the transaction.

Do not end up with:

School created
but
Token still unused


or:

Token marked used
but
School not created


or:

School created
but
Subscription missing


==================================================
DOUBLE SUBMISSION / RACE CONDITION
==================================================

Two requests can potentially reach the same token simultaneously.

Expected behavior:

Request A:
SUCCESS

Request B:
FAIL because token is already used


There must never be:

2 schools
2 referrals
2 subscriptions
2 commissions

created from one one-time token.


==================================================
DIRECT SIGNUP MUST REMAIN UNCHANGED
==================================================

A normal school that does NOT use a referral link must continue using:

app.eduplexo.com

↓

Create Account

↓

Normal pricing options

↓

Select plan

↓

Create School

↓

Create School Admin


This flow must not be broken by referral logic.


==================================================
REFERRAL SIGNUP MUST BE DIFFERENT
==================================================

Referral:

app.eduplexo.com/invite/{token}

↓

Validate token

↓

Show ONLY assigned plan

↓

No normal pricing-card selection

↓

Create School

↓

Create School Admin

↓

Create assigned Subscription

↓

Attach Publisher

↓

Create Referral

↓

Token USED


==================================================
EXISTING SCHOOL CASE
==================================================

If an existing school opens a referral token:

Do NOT create duplicate school.

Require secure authentication/verification.

If allowed by business rules:

Existing School
    ↓
Claim Referral
    ↓
Attach Publisher
    ↓
Apply/record referral subscription offer if permitted
    ↓
Token USED


Do not silently replace an existing active subscription without inspecting current billing/subscription rules.

If existing subscriptions make referral-plan switching unsafe, block the operation with a clear message rather than corrupting billing state.


==================================================
DATABASE COMPATIBILITY
==================================================

Inspect existing plan/subscription tables before adding fields.

Avoid duplicate:

- plans
- pricing tables
- subscriptions
- users
- schools
- publishers
- referral records

Use migrations.

Do not reset the database.

Do not drop existing production data.


==================================================
OWNER ARCHITECTURE CHECK
==================================================

Before declaring this feature complete, verify that referral onboarding NEVER creates Owner.

Referral onboarding must produce:

School
+
School Admin

The referral system must not require:

Owner
Campus
Owner ID
Owner permissions


==================================================
ADMIN UI
==================================================

The new referral pricing UI must match the existing Admin design system.

Do not redesign the whole Admin portal.

The Generate Referral Link modal should be polished and clear.

Suggested structure:

Create Referral Link

Publisher
ABC Education

Plan
[ Professional ]

Default price
Rs. 15,000 / month

Referral price
[ Rs. 12,000 ]

Billing
Monthly

[ Generate Link ]


Then one-time result:

Referral Link Created

ABC Education
Professional
Rs. 12,000/month

This link will only be shown once.

[Copy Link]

[Done]


==================================================
SCHOOL REFERRAL UI
==================================================

Referral landing should be visually polished.

Example:

EduPlexo

Your EduPlexo Plan

Professional

Rs. 12,000 / month

Your referral offer includes:

Student Management
Teacher Management
Classes & Timetable
Attendance
Homework & Exams
Fees
...

[Create School Account]


The exact feature list should come from the existing plan configuration.

Do not hardcode fake modules.


==================================================
SUBSCRIPTION PAGE BEHAVIOR
==================================================

For a referral-created school:

Show:

Current Plan
Professional

Monthly Price
Rs. 12,000

Student Capacity
500

Included Modules
...

Do NOT show public upgrade/plan-selection cards unless existing business logic explicitly requires them.

The school should understand that its current subscription was established through a referral offer.


==================================================
ADMIN VERIFICATION
==================================================

Admin must be able to verify:

Publisher
Referral Token
Plan
Referral Price
School
Subscription
Referral Status
Commission Status


Example:

ABC Education

Referral #001
School: Oak Academy
Plan: Professional
Price: Rs. 12,000/month
Status: Active
Commission: Pending


==================================================
TESTING
==================================================

Perform end-to-end testing.

TEST 1 — Direct Signup

New user
    ↓
Create Account
    ↓
Normal pricing cards
    ↓
Select plan
    ↓
Create School
    ↓
Create School Admin


Verify:
No Owner.


TEST 2 — Referral Signup

Admin
    ↓
Create Publisher
    ↓
View Publisher
    ↓
Generate Referral Link
    ↓
Select Professional
    ↓
Set Referral Price = Rs. 12,000
    ↓
Generate Token
    ↓
Copy link
    ↓
Done
    ↓
Raw token unavailable afterwards


School:
Open referral link
    ↓
Show Professional only
    ↓
Show Rs. 12,000/month
    ↓
No public pricing cards
    ↓
Create School
    ↓
Create School Admin
    ↓
Create Subscription
    ↓
Publisher assignment
    ↓
Referral created
    ↓
Token USED


TEST 3 — Plan Snapshot

Create Token:
Professional
Rs. 12,000

Change Professional live price to:
Rs. 15,000

Use old token.

Expected school price:

Rs. 12,000

NOT Rs. 15,000.


TEST 4 — Multiple Tokens

Same Publisher:

Token 001
Professional
Rs. 12,000

Token 002
Custom
Rs. 18,500

Verify they remain separate.


TEST 5 — Token Reuse

Use Token 001.

Try Token 001 again.

Expected:
FAIL.


TEST 6 — Publisher Isolation

Publisher A must not see Publisher B's referrals/schools/prices.


TEST 7 — Owner Regression

Search referral onboarding.

Expected:
No Owner creation.


TEST 8 — School Subscription

Referral school logs in.

Open Subscription.

Expected:

Assigned plan visible
Assigned monthly price visible
Correct modules visible
No irrelevant public plan cards


TEST 9 — Existing Functionality

Verify:

- Teachers
- Students
- Classes
- Fees
- Attendance
- Exams
- Existing subscription functionality
- School dashboard
- School settings

still work.


==================================================
IMPORTANT DEPLOYMENT RULES
==================================================

Reuse the current backend.

Publisher frontend may be deployed on Vercel.

Existing school app remains at:

app.eduplexo.com

Referral routes must work correctly in production.

Do not hardcode localhost URLs.


==================================================
IMPLEMENTATION PROCESS
==================================================

PHASE 1 — INSPECT

Before coding:

Inspect existing:

- Publisher implementation
- Referral implementation
- Pricing plans
- Custom plans
- Subscription creation
- Signup
- School creation
- School Admin creation
- Owner removal state
- Database schema


PHASE 2 — REPORT

Tell me:

Already implemented
Partially implemented
Broken
Missing

Especially verify:

Does referral currently create School Admin?
Does referral currently create Owner?
Does referral currently select a plan?
Does referral currently snapshot price?
Does referral currently create a subscription?
Does subscription reflect referral pricing?
Are other pricing cards hidden?
Is token one-time?
Is token publisher-specific?


PHASE 3 — BACKEND

Implement/fix:

- referral offer
- token pricing snapshot
- plan association
- secure token
- school assignment
- subscription creation
- atomic token consumption


PHASE 4 — ADMIN

Implement/fix:

- Publisher
- Publisher Detail
- Generate Referral Link
- Plan selection
- Referral price
- One-time secret modal
- Referral history


PHASE 5 — SCHOOL APP

Implement/fix:

- /invite/:token
- referral validation
- locked plan presentation
- no public pricing selection
- School + School Admin signup
- subscription creation


PHASE 6 — PUBLISHER PORTAL

Implement/fix:

- Dashboard
- My Schools
- Referrals
- Plan
- Monthly referral value
- Commission status where configured


PHASE 7 — TEST

Run complete end-to-end tests.


==================================================
FINAL ARCHITECTURE
==================================================

DIRECT SIGNUP

app.eduplexo.com
    ↓
Create Account
    ↓
Pricing Cards
    ↓
Select Plan
    ↓
Create School
    ↓
Create School Admin
    ↓
Subscription


PUBLISHER REFERRAL

Admin
    ↓
Publisher
    ↓
Generate Referral Link
    ↓
Select Plan
    ↓
Set Referral Price
    ↓
Secure One-Time Token
    ↓
app.eduplexo.com/invite/{token}
    ↓
Assigned Plan Only
    ↓
No Public Pricing Cards
    ↓
Create School
    ↓
Create School Admin
    ↓
Create Subscription
    ↓
Publisher-School Assignment
    ↓
Referral Record
    ↓
Token USED


==================================================
FINAL BUSINESS RULE
==================================================

The referral token represents:

WHO referred the school
+
WHICH plan the school receives
+
WHAT monthly referral price was agreed
+
WHICH school eventually consumed the token


Therefore:

Publisher
   +
Plan
   +
Price Snapshot
   +
One-Time Token
   ↓
School
   ↓
School Admin
   ↓
Subscription


There is NO Owner anywhere in this flow.

There is NO multi-campus creation.

There is NO Owner account.

There is NO referral plan selection after the referral context has been established.

The referral offer must be locked server-side.


==================================================
FINAL RESPONSE FORMAT
==================================================

Before implementation report:

1. Existing referral functionality
2. Existing Publisher functionality
3. Existing pricing/plan architecture
4. Existing subscription architecture
5. Existing school signup architecture
6. Existing School Admin creation
7. Whether referral currently creates Owner or School Admin
8. Whether referral pricing is already supported
9. Files/modules that need changes
10. Risks and migration concerns


After implementation report:

1. What already existed
2. What was fixed
3. What was newly implemented
4. Database changes
5. Token behavior
6. Plan/price snapshot behavior
7. School referral flow
8. School Admin creation
9. Subscription behavior
10. Admin UI
11. Publisher UI
12. Owner verification
13. Tests executed
14. End-to-end result
15. Remaining issues

Do not claim something is verified unless it was actually tested or inspected in the real codebase.