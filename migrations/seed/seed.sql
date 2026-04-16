-- GradConnect Seed Data
-- Run with: make db/seed
-- Or: psql $GRADCONNECT_DB_DSN -f ./migrations/seed.sql

-- Clear existing data (in dependency order)
TRUNCATE employer CASCADE;

-- ============================================================================
-- Employers
-- ============================================================================

INSERT INTO employer (id, name, slug, industry, size, hq_location, offices, logo_url, overview, culture, website, social_links, is_verified)
VALUES
(
    'a1b2c3d4-1111-4000-8000-000000000001',
    'Access Bank',
    'access-bank',
    'Banking & Finance',
    '1000+',
    'Lagos',
    '[{"city": "Lagos", "state": "Lagos", "address": "Danmole St, Victoria Island"}, {"city": "Abuja", "state": "FCT", "address": "Plot 999, Cadastral Zone"}]'::jsonb,
    'https://cdn.gradconnect.ng/logos/access-bank.png',
    'Access Bank Plc is one of Nigeria''s largest commercial banks by total assets. The bank serves over 50 million customers across Africa and the United Kingdom, offering a full range of retail, commercial, and corporate banking services.',
    'Access Bank values innovation, integrity, and excellence. The graduate programme emphasises rotational learning across departments, mentorship from senior leaders, and early responsibility on real client engagements.',
    'https://www.accessbankplc.com',
    '{"linkedin": "https://linkedin.com/company/access-bank", "twitter": "https://twitter.com/myaborteam"}'::jsonb,
    true
),
(
    'a1b2c3d4-2222-4000-8000-000000000002',
    'PricewaterhouseCoopers (PwC)',
    'pwc-nigeria',
    'Professional Services',
    '1000+',
    'Lagos',
    '[{"city": "Lagos", "state": "Lagos", "address": "Landmark Towers, Victoria Island"}, {"city": "Abuja", "state": "FCT", "address": "Plot 1164, Cadastral Zone"}]'::jsonb,
    'https://cdn.gradconnect.ng/logos/pwc.png',
    'PwC Nigeria is a member firm of the PricewaterhouseCoopers global network. The firm provides assurance, advisory, and tax services to leading organisations in Nigeria across all major industries.',
    'PwC fosters a culture of continuous learning and professional development. Graduate associates are enrolled in professional certification programmes (ICAN, ACCA) from day one, with structured study support and exam leave.',
    'https://www.pwc.com/ng',
    '{"linkedin": "https://linkedin.com/company/pwc-nigeria", "twitter": "https://twitter.com/PwC_Nigeria"}'::jsonb,
    true
),
(
    'a1b2c3d4-3333-4000-8000-000000000003',
    'Dangote Group',
    'dangote-group',
    'Manufacturing & FMCG',
    '1000+',
    'Lagos',
    '[{"city": "Lagos", "state": "Lagos", "address": "Alfred Rewane Road, Ikoyi"}, {"city": "Obajana", "state": "Kogi", "address": "Dangote Cement Plant"}]'::jsonb,
    'https://cdn.gradconnect.ng/logos/dangote.png',
    'Dangote Group is the largest industrial conglomerate in West Africa and one of the largest on the African continent. The group operates across cement, sugar, salt, flour, and petrochemicals, with operations in 10 African countries.',
    'Dangote offers a fast-paced, operationally focused environment. Graduate trainees rotate through factory operations, supply chain, and commercial functions. The programme values hands-on problem solving and operational excellence.',
    'https://www.dangote.com',
    '{"linkedin": "https://linkedin.com/company/dangote-group"}'::jsonb,
    true
),
(
    'a1b2c3d4-4444-4000-8000-000000000004',
    'Shell Nigeria',
    'shell-nigeria',
    'Oil & Gas',
    '1000+',
    'Lagos',
    '[{"city": "Lagos", "state": "Lagos", "address": "Freeman House, Marina"}, {"city": "Port Harcourt", "state": "Rivers", "address": "Shell Industrial Area, Rumuobiakani"}]'::jsonb,
    'https://cdn.gradconnect.ng/logos/shell.png',
    'Shell Petroleum Development Company of Nigeria Limited (SPDC) is the pioneer and largest oil and gas exploration and production company in Nigeria. Shell has been operating in Nigeria since 1937 and produces approximately 10% of Nigeria''s total oil output.',
    'Shell offers a structured graduate development programme with international exposure. Trainees work on real projects in engineering, finance, HR, and commercial roles, with a strong emphasis on safety culture and technical excellence.',
    'https://www.shell.com.ng',
    '{"linkedin": "https://linkedin.com/company/shell", "twitter": "https://twitter.com/Shell_Nigeria"}'::jsonb,
    true
),
(
    'a1b2c3d4-5555-4000-8000-000000000005',
    'Flutterwave',
    'flutterwave',
    'Technology & Fintech',
    '201-1000',
    'Lagos',
    '[{"city": "Lagos", "state": "Lagos", "address": "Herbert Macaulay Way, Sabo, Yaba"}]'::jsonb,
    'https://cdn.gradconnect.ng/logos/flutterwave.png',
    'Flutterwave is a leading African fintech company that provides payment infrastructure for global merchants and payment service providers. The company processes millions of transactions and serves businesses across 34 African countries.',
    'Flutterwave thrives on speed, ownership, and building for Africa. The engineering and product teams ship fast, with a startup culture that gives early-career talent real ownership over features and systems from day one.',
    'https://www.flutterwave.com',
    '{"linkedin": "https://linkedin.com/company/flutterwave", "twitter": "https://twitter.com/theaborteam"}'::jsonb,
    false
);

-- ============================================================================
-- Assessment Profiles
-- ============================================================================

TRUNCATE assessment_profile CASCADE;

INSERT INTO assessment_profile (id, employer_id, programme_type, stages, aptitude_test_provider, interview_format, timeline_weeks, prep_guide)
VALUES
(
    'b1b2c3d4-1111-4000-8000-000000000001',
    'a1b2c3d4-1111-4000-8000-000000000001', -- Access Bank
    'Graduate Trainee',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV upload, cover letter, and basic screening questions", "order": 1},
        {"stage_name": "Aptitude Test", "stage_type": "test", "description": "SHL numerical and verbal reasoning, 45 minutes timed", "order": 2},
        {"stage_name": "Video Interview", "stage_type": "interview", "description": "Pre-recorded answers to 5 competency questions via HireVue", "order": 3},
        {"stage_name": "Assessment Centre", "stage_type": "assessment", "description": "Full-day session: group exercise, case study presentation, and panel interview", "order": 4}
    ]'::jsonb,
    'SHL',
    'competency-based',
    8,
    '## How to prepare\n\nStart with SHL-style numerical and verbal reasoning practice. The official SHL app is the best resource. For the video interview, use the STAR method for every answer. At the assessment centre, be collaborative in the group exercise — assessors watch teamwork as much as individual contribution.'
),
(
    'b1b2c3d4-1112-4000-8000-000000000002',
    'a1b2c3d4-1111-4000-8000-000000000001', -- Access Bank (second programme)
    'Summer Internship',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV and transcript upload", "order": 1},
        {"stage_name": "Aptitude Test", "stage_type": "test", "description": "Shorter SHL test — numerical reasoning only, 25 minutes", "order": 2},
        {"stage_name": "Panel Interview", "stage_type": "interview", "description": "30-minute interview with two managers, competency and motivation questions", "order": 3}
    ]'::jsonb,
    'SHL',
    'competency-based',
    4,
    '## How to prepare\n\nThe internship process is shorter and less intense than the graduate programme. Focus on numerical reasoning practice and have clear answers for why banking and why Access Bank.'
),
(
    'b1b2c3d4-2222-4000-8000-000000000003',
    'a1b2c3d4-2222-4000-8000-000000000002', -- PwC
    'Graduate Associate',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV, academic transcripts, and ICAN/ACCA status", "order": 1},
        {"stage_name": "Aptitude Test", "stage_type": "test", "description": "Korn Ferry numerical, verbal, and logical reasoning — 60 minutes total", "order": 2},
        {"stage_name": "Group Exercise", "stage_type": "assessment", "description": "Case study discussion in groups of 6, observed by partners", "order": 3},
        {"stage_name": "Partner Interview", "stage_type": "interview", "description": "One-on-one with a partner, mix of technical accounting and behavioural questions", "order": 4}
    ]'::jsonb,
    'Korn Ferry',
    'technical',
    10,
    '## How to prepare\n\nKorn Ferry tests are tougher than SHL — practice logical reasoning specifically. For the partner interview, brush up on IFRS basics and have a view on a recent audit controversy in Nigeria. The group exercise rewards structured thinking and clear communication over dominance.'
),
(
    'b1b2c3d4-3333-4000-8000-000000000004',
    'a1b2c3d4-3333-4000-8000-000000000003', -- Dangote
    'Graduate Trainee',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV and cover letter via careers portal", "order": 1},
        {"stage_name": "Aptitude Test", "stage_type": "test", "description": "Custom in-house test — numerical, verbal, and abstract reasoning", "order": 2},
        {"stage_name": "Technical Interview", "stage_type": "interview", "description": "Role-specific technical questions with department heads", "order": 3},
        {"stage_name": "Final Interview", "stage_type": "interview", "description": "HR and senior management panel, focus on cultural fit and resilience", "order": 4}
    ]'::jsonb,
    'Custom',
    'technical',
    6,
    '## How to prepare\n\nDangote values operational mindset and willingness to work in non-Lagos locations (Obajana, Apapa). Be ready to discuss why you want to work in manufacturing/FMCG. The technical interview is role-specific — engineers get engineering questions, finance gets accounting questions.'
),
(
    'b1b2c3d4-4444-4000-8000-000000000005',
    'a1b2c3d4-4444-4000-8000-000000000004', -- Shell
    'Graduate Programme',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV, degree details, and Shell online questionnaire", "order": 1},
        {"stage_name": "Shell Online Assessment", "stage_type": "test", "description": "Cognitive ability, situational judgement, and a Shell-specific values assessment", "order": 2},
        {"stage_name": "Video Interview", "stage_type": "interview", "description": "HireVue interview with 6 competency questions aligned to Shell values (honesty, integrity, respect)", "order": 3},
        {"stage_name": "Assessment Day", "stage_type": "assessment", "description": "Group exercise, individual case study, and technical interview with senior engineers or managers", "order": 4}
    ]'::jsonb,
    'Custom',
    'case study',
    12,
    '## How to prepare\n\nShell''s process is one of the longest and most rigorous in Nigeria. Study the Shell Graduate website thoroughly — their values framework drives every stage. The online assessment includes a unique situational judgement test. For the assessment day, expect a technical deep-dive in your discipline area.'
),
(
    'b1b2c3d4-5555-4000-8000-000000000006',
    'a1b2c3d4-5555-4000-8000-000000000005', -- Flutterwave
    'Graduate Engineer',
    '[
        {"stage_name": "Online Application", "stage_type": "form", "description": "CV and GitHub/portfolio link", "order": 1},
        {"stage_name": "Take-Home Assignment", "stage_type": "test", "description": "Build a small API or frontend feature — 48 hours to complete", "order": 2},
        {"stage_name": "Technical Interview", "stage_type": "interview", "description": "Live coding session and system design discussion with senior engineers", "order": 3},
        {"stage_name": "Culture Fit", "stage_type": "interview", "description": "Conversation with the hiring manager about ownership, speed, and building for Africa", "order": 4}
    ]'::jsonb,
    NULL,
    'technical',
    3,
    '## How to prepare\n\nFlutterwave cares about shipping ability over credentials. Make sure your GitHub has real projects, not just tutorials. The take-home is evaluated on code quality, test coverage, and documentation — not just whether it works. For the system design round, be ready to discuss trade-offs at scale.'
);