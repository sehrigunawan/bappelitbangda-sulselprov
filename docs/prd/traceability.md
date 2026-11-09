Status: generated
Last reviewed: 2026-11-02

# Traceability: PRD requirement to issues and tests

Generated from `docs/prd/PRD.md` by `scripts/gen-traceability.sh`. Fill the issue, test and status columns by hand.
`make docs-check` fails if the id list here differs from the PRD (45 FR, 17 NFR).

| Id | Title (PRD words) | Priority | Phase | Status | Issue | Test | Notes (OQ dependencies) |
|---|---|---|---|---|---|---|---|
| FR-01 | The system must display a home page containing a banner slider, three PPID form shortcuts, a message from the head of the agency, the seven latest news items, a survey block, and the address and contact details. | Must | 1 MVP | not started |  |  |  |
| FR-02 | The system must provide navigation up to three levels deep, including the mobile menu, managed from a single data source. | Must | 1 MVP | not started |  |  |  |
| FR-03 | The system must take the address, phone, email, and social media links from a single site setting for the topbar, footer, and contact page. | Must | 1 MVP | not started |  |  |  |
| FR-44 | The system must display public visitor statistics (today, this month, this year) in the footer of every public page. | Must | 1 MVP | not started |  |  |  |
| FR-04 | The system should provide global search across publications, pages, and documents with results grouped by type. | Should | 2 Strengthening | not started |  |  |  |
| FR-45 | The system should allow the Humas Editor to designate one static page (FR-05) as the main hero of the home page, showing the title, summary, main image, and a link to that page at the top of the home page. | Should | 2 Strengthening | not started |  |  |  |
| FR-05 | The system must provide static pages with a rich text editor (title, lists, tables, images, attachments), slug, draft or published status, and position in the menu. | Must | 1 MVP | not started |  |  |  |
| FR-07 | The system must display the staff list (name, position, rank, education, division) with search, division filter, sorting, and pagination. | Must | 1 MVP | not started |  |  |  |
| FR-06 | The system should manage periodic content (organizational structure, officials' profiles, staffing and financial statistics) as a series with effective dates, with a page showing the current version and a list of archives. | Should | 2 Strengthening | not started |  |  | OQ-07 |
| FR-08 | The system should import and update the staff list from CSV files and normalize education and rank values to a pick list. | Should | 2 Strengthening | not started |  |  |  |
| FR-09 | The system should supplement image-based content with alternative text and present statistics tables as structured data displayed as HTML tables. | Should | 2 Strengthening | not started |  |  | OQ-06 |
| FR-10 | The system must display the publication list per category (10 categories) as cards containing image, title, category, date, and excerpt, with pagination. | Must | 1 MVP | not started |  |  |  |
| FR-11 | The system must display publication detail: title, category, date, view count, rich text content with images, tags, and related publications. | Must | 1 MVP | not started |  |  | OQ-01 |
| FR-13 | The system must display a sidebar containing categories with post counts, a search box, and "Popular" calculated from views in the last 30 days. | Must | 1 MVP | not started |  |  |  |
| FR-12 | The system should provide a tag page showing the list of publications per tag. | Should | 2 Strengthening | not started |  |  |  |
| FR-14 | The system can provide a photo gallery (albums) and videos (YouTube links). | Could | 3 Additions | not started |  |  | OQ-09 |
| FR-15 | The system must display the document catalog: each category shows its download count, and the list per category contains title, update date, download count, View and Download buttons, and pagination. | Must | 1 MVP | not started |  |  |  |
| FR-16 | The system should provide document search and filters across categories by keyword, category, and year. | Should | 2 Strengthening | not started |  |  |  |
| FR-17 | The system should build a hierarchical document taxonomy (group, then category) agreed with the content owner, with old category IDs mapped to new categories. | Should | 2 Strengthening | not started |  |  | OQ-08 |
| FR-18 | The system should record the download counter on the server when a file is downloaded (one count per visitor per file per day) and migrate the old values as the starting value. | Should | 2 Strengthening | not started |  |  |  |
| FR-19 | The system must display the pages Public Information List, Periodic Information, Information Available at Any Time, Immediately Available Information, and Excluded Information from the Documents module according to the information-type marker, not manual tables and links. | Must | 1 MVP | not started |  |  |  |
| FR-20 | The system must provide an information request form with the applicant's identity (name, KTP, email, phone, address, occupation), the information user's identity (name, KTP, email, phone), details of the information, and purpose of use. | Must | 1 MVP | not started |  |  | OQ-03 |
| FR-21 | The system must provide an objection submission form with the reason for the information request, the applicant's identity, an option of not represented or represented (representative's identity and a power-of-attorney letter as PDF or image), seven grounds for objection, and a short chronology. | Must | 1 MVP | not started |  |  |  |
| FR-22 | The system must provide a complaint form about abuse of authority with the complainant's identity, the official's name and position, an explanation, and evidence uploads (.pdf, .jpg, .zip, .rar). | Must | 1 MVP | not started |  |  | OQ-12 |
| FR-24 | The system must provide a PPID officer queue containing submissions by type and status, with functions to change status, record answers and attachments, and a flag for the 10-working-day deadline and the 7-working-day extension for information requests. | Must | 1 MVP | not started |  |  | OQ-02 |
| FR-23 | The system should give a ticket number to every submission of FR-20 to FR-22 and allow status tracking (received, verified, processed, extended, answered, rejected) through a public page, with a notification email if the address is valid. | Should | 2 Strengthening | not started |  |  | OQ-11 |
| FR-25 | The system must provide a public information service survey with respondent identity (name, institution, email, age, phone, date, type of service) and 17 questions on a scale from Very Good to Not Good. | Must | 1 MVP | not started |  |  |  |
| FR-26 | The system must provide an IKM survey with respondent identity (OPD, name, age, gender, education, phone, date, position, type of service), 24 items on a scale of 1 to 4 with a reason column, a conditional cost question, and an input for service innovation. | Must | 1 MVP | not started |  |  |  |
| FR-27 | The system must display poll results as a percentage per option calculated from the actual number of respondents per period, with the correct page title, and allow the admin to download responses as CSV. | Must | 1 MVP | not started |  |  |  |
| FR-28 | The system must display the Applications page containing cards for applications and related services (logo, name, short description, destination URL) in an order set by the admin. | Must | 1 MVP | not started |  |  |  |
| FR-29 | The system must provide a contact page containing address, phone, email, location map, and a response form that goes to the admin's message inbox. | Must | 1 MVP | not started |  |  |  |
| FR-30 | The system can display an activity calendar on the home page from agenda entries input by the admin. | Could | 3 Additions | not started |  |  | OQ-05 |
| FR-31 | The system can display a prayer times (sholat) widget on the home page. | Could | 3 Additions | not started |  |  | OQ-05 |
| FR-32 | The system can provide internal visit statistics (daily, monthly, most popular pages) on the admin dashboard. | Could | 3 Additions | not started |  |  |  |
| FR-33 | The system must provide admin login with roles and per-module permissions for five roles: Super Admin, Humas Editor, Document Operator, PPID Officer, and Leadership (read-only); one user can hold more than one role, and the effective permissions are the union of the permissions of all their roles. | Must | 1 MVP | done | #26, #28 | api/internal/server/auth_*_test.go, web/src/routes/admin/ | OQ-10 |
| FR-34 | The system must allow the Humas Editor to manage publications: write, edit, save as draft, schedule, publish, and set category, tags, and main image. | Must | 1 MVP | not started |  |  |  |
| FR-35 | The system must allow the Humas Editor to manage static pages and the navigation menu. | Must | 1 MVP | not started |  |  |  |
| FR-36 | The system must allow the Document Operator to manage documents and categories: upload files, fill in title and date, choose category and information-type marker, and bulk upload. | Must | 1 MVP | not started |  |  |  |
| FR-37 | The system must allow the Humas Editor and Super Admin to manage sliders, applications, staff, surveys (questions and periods), holidays for working-day calculation, and site settings. | Must | 1 MVP | not started |  |  |  |
| FR-38 | The system must provide a media library for authorized administrators: upload images and files, type and size validation, automatic derived sizes (thumbnails), and alternative text for images. | Must | 1 MVP | not started |  |  |  |
| FR-39 | The system should record an activity log: who changed what and when on content and settings. | Should | 2 Strengthening | not started |  |  |  |
| FR-40 | The system can provide a summary for leadership: number and status of PPID requests, average response time, and survey results per period. | Won't | Deferred (Won't) | not started |  |  |  |
| FR-41 | The system must redirect (301) all old URL patterns (/pages/{slug}, /detailpost/{slug}, /category/{slug}/{page}, /all-dokumen/{id}, /tag/{slug}, /content/uploads/, /content/new\\\_directory/2024/) to their new equivalents. | Must | 1 MVP | not started |  |  |  |
| FR-42 | The system must generate sitemap.xml and robots.txt automatically and fill in the title, description, canonical, and Open Graph on every page. | Must | 1 MVP | not started |  |  |  |
| FR-43 | The system should display 404 and 500 pages containing a search box and a link to the home page. | Should | 2 Strengthening | not started |  |  |  |
| NFR-01 | Public page loading speed | Must | MVP gate | not started |  |  |  |
| NFR-02 | API capacity and response time | Should | Should (not a release gate) | not started |  |  |  |
| NFR-03 | WCAG 2.1 AA compliance on all public pages | Must | MVP gate | not started |  |  |  |
| NFR-04 | Rendering and metadata | Must | MVP gate | not started |  |  |  |
| NFR-05 | Application security | Must | MVP gate | in progress | #26 | api/internal/server/auth_*_test.go |  |
| NFR-06 | File uploads | Must | MVP gate | not started |  |  | OQ-12, OQ-04 |
| NFR-07 | Public form protection | Must | MVP gate | not started |  |  |  |
| NFR-08 | Personal data protection under Law Number 27 of 2022 | Must | MVP gate | not started |  |  | OQ-15 |
| NFR-09 | Availability | Should | Should (not a release gate) | not started |  |  |  |
| NFR-10 | Backup and recovery | Must | MVP gate | not started |  |  |  |
| NFR-11 | Responsiveness and browsers | Must | MVP gate | not started |  |  |  |
| NFR-12 | Migration integrity | Must | MVP gate | not started |  |  |  |
| NFR-13 | URL compatibility | Must | MVP gate | not started |  |  |  |
| NFR-14 | Code quality and documentation | Should | Should (not a release gate) | not started |  |  |  |
| NFR-15 | Observability | Should | Should (not a release gate) | not started |  |  |  |
| NFR-16 | Language and format | Should | Should (not a release gate) | not started |  |  |  |
| NFR-17 | Encryption of data in transit and at rest | Must | MVP gate | in progress | #26 | api/internal/server/auth_*_test.go | OQ-16, OQ-17 |
