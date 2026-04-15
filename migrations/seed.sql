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