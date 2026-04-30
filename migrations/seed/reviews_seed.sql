-- ============================================================================
-- Seed: users + approved reviews
-- Run AFTER the main seed.sql (needs employer IDs to exist).
--
-- Creates:
--   * 6 app_user rows (so reviews have real user_id FKs)
--   * 11 reviews across 4 employers, all already `approved` for public display
--   * Recalculates each employer's aggregate ratings from the reviews
-- ============================================================================

TRUNCATE review CASCADE;
TRUNCATE app_user CASCADE;

-- ============================================================================
-- Users
-- Password hashes are bcrypt for "password123" — dev-only, never use in prod.
-- All users are email-verified with review:submit permissions granted
-- separately via user_permission (if applicable in your app).
-- ============================================================================

INSERT INTO app_user (id, email, password_hash, first_name, last_name, auth_provider, email_verified, degree_discipline, graduation_year, target_industries, preferred_locations)
VALUES
(
    'd0000001-0000-4000-8000-000000000001',
    'ayomide.okafor@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Ayomide',
    'Okafor',
    'email',
    true,
    'Economics',
    2024,
    ARRAY['Banking & Finance', 'Professional Services'],
    ARRAY['Lagos', 'Abuja']
),
(
    'd0000002-0000-4000-8000-000000000002',
    'chidera.adeniyi@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Chidera',
    'Adeniyi',
    'email',
    true,
    'Accounting',
    2024,
    ARRAY['Professional Services'],
    ARRAY['Lagos']
),
(
    'd0000003-0000-4000-8000-000000000003',
    'tunde.balogun@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Tunde',
    'Balogun',
    'email',
    true,
    'Mechanical Engineering',
    2023,
    ARRAY['Oil & Gas', 'Manufacturing & FMCG'],
    ARRAY['Port Harcourt', 'Lagos']
),
(
    'd0000004-0000-4000-8000-000000000004',
    'nkem.eze@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Nkem',
    'Eze',
    'google',
    true,
    'Petroleum Engineering',
    2024,
    ARRAY['Oil & Gas'],
    ARRAY['Port Harcourt']
),
(
    'd0000005-0000-4000-8000-000000000005',
    'fatima.abubakar@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Fatima',
    'Abubakar',
    'email',
    true,
    'Computer Science',
    2025,
    ARRAY['Technology & Fintech'],
    ARRAY['Lagos']
),
(
    'd0000006-0000-4000-8000-000000000006',
    'seun.adebayo@example.com',
    '$2a$10$7gDj.xRQXqGzYyF4Y5Tf4eyVx5v9Z5n5vZ5n5vZ5n5vZ5n5vZ5n5v',
    'Seun',
    'Adebayo',
    'email',
    true,
    'Finance',
    2023,
    ARRAY['Banking & Finance'],
    ARRAY['Lagos']
);

-- ============================================================================
-- Reviews — all pre-approved for public display
-- Spread across Access Bank, PwC, Shell, and Flutterwave
-- Outcomes: offer, rejected, waitlisted, withdrew
-- Difficulty + experience ratings vary for realistic aggregates
-- ============================================================================

INSERT INTO review (id, employer_id, user_id, programme_name, application_year, outcome, stage_breakdown, difficulty_rating, experience_rating, tips, degree_discipline, university, status, created_at)
VALUES
-- ============== Access Bank — 4 reviews ==============
(
    'e0000001-0000-4000-8000-000000000001',
    'a1b2c3d4-1111-4000-8000-000000000001',
    'd0000001-0000-4000-8000-000000000001',
    'Graduate Trainee Programme 2025',
    2025,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "Straightforward CV upload and four short written answers. No trick questions.", "tips": "Keep answers under 200 words each and focus on concrete achievements, not generic traits."},
        {"stage_name": "SHL Aptitude Test", "description": "Combined numerical and verbal reasoning. I had about two weeks between application and test.", "tips": "Use the official SHL practice app. The free online tests underestimate the real difficulty."},
        {"stage_name": "Video Interview", "description": "Five pre-recorded questions via HireVue. 30 seconds to prepare, 2 minutes to answer each.", "tips": "Record yourself beforehand. Your camera presence matters more than you think."},
        {"stage_name": "Assessment Centre", "description": "Full day in Victoria Island. Group case study on financial inclusion, individual presentation, panel interview.", "tips": "In the group exercise, contribute early and listen genuinely. Assessors watch group dynamics closely."}
    ]'::jsonb,
    4,
    4,
    E'Start preparing at least two weeks before the aptitude test. The assessment centre is intense but fair — they genuinely want you to succeed. Ask thoughtful questions at the end of the panel interview.',
    'Economics',
    'University of Lagos',
    'approved',
    '2025-07-15 10:00:00+01'
),
(
    'e0000001-0000-4000-8000-000000000002',
    'a1b2c3d4-1111-4000-8000-000000000001',
    'd0000006-0000-4000-8000-000000000006',
    'Graduate Trainee Programme 2024',
    2024,
    'rejected',
    '[
        {"stage_name": "Online Application", "description": "Submitted on the last day. Probably hurt my chances.", "tips": "Apply early. Access Bank processes applications in batches, and early ones get more attention."},
        {"stage_name": "SHL Aptitude Test", "description": "Struggled with the verbal reasoning section. Ran out of time on the last three questions.", "tips": "Practice pacing with a timer. Getting 80% correct on 20 questions beats 90% on 15."},
        {"stage_name": "Video Interview", "description": "Didn''t get past this stage. Got a rejection email two weeks later.", "tips": "I later realised I was too rehearsed. Real conversation beats memorised answers."}
    ]'::jsonb,
    4,
    3,
    E'Don''t make the mistakes I did. Apply early, practice pacing on the aptitude test, and be yourself on the video interview. Reapplying next cycle with lessons learned.',
    'Finance',
    'Covenant University',
    'approved',
    '2024-09-20 14:30:00+01'
),
(
    'e0000001-0000-4000-8000-000000000003',
    'a1b2c3d4-1111-4000-8000-000000000001',
    'd0000002-0000-4000-8000-000000000002',
    'Summer Internship 2024',
    2024,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "CV and transcript upload. Short, no written answers required.", "tips": "Include any banking society involvement or finance coursework prominently."},
        {"stage_name": "Aptitude Test", "description": "Shorter than the full graduate test — just numerical reasoning, 25 minutes.", "tips": "Brush up on percentages, ratios, and currency conversion. Core banking maths."},
        {"stage_name": "Panel Interview", "description": "30 minutes with a retail banking manager and HR. Mostly motivation and competency questions.", "tips": "Have a clear answer for why banking specifically, not just Access Bank."}
    ]'::jsonb,
    3,
    5,
    E'The internship is less intense than the full graduate scheme but still selective. Have strong answers ready for why banking and why Access Bank. The panel interviewers are friendly and give you time to think.',
    'Accounting',
    'University of Ibadan',
    'approved',
    '2024-08-05 09:15:00+01'
),
(
    'e0000001-0000-4000-8000-000000000004',
    'a1b2c3d4-1111-4000-8000-000000000001',
    'd0000005-0000-4000-8000-000000000005',
    'Graduate Trainee Programme 2025',
    2025,
    'waitlisted',
    '[
        {"stage_name": "Online Application", "description": "Submitted CV and completed all four written questions.", "tips": "Tailor each answer to the role — generic answers signal low effort."},
        {"stage_name": "SHL Aptitude Test", "description": "Passed comfortably after two weeks of practice.", "tips": "The SHL app''s numerical practice is the closest to the real test."},
        {"stage_name": "Video Interview", "description": "Went well from my perspective.", "tips": "STAR method for every answer. Keep examples recent and relevant."},
        {"stage_name": "Assessment Centre", "description": "Made it to the assessment centre but didn''t get an offer. Placed on the waitlist.", "tips": "The case study rewards structure. Start with a framework before diving into details."}
    ]'::jsonb,
    5,
    4,
    E'Getting waitlisted is not a rejection. Two of my batch were eventually offered roles when others declined. Stay in touch with the HR team and reply promptly if they reach out.',
    'Computer Science',
    'Obafemi Awolowo University',
    'approved',
    '2025-08-01 11:45:00+01'
),

-- ============== PwC — 3 reviews ==============
(
    'e0000002-0000-4000-8000-000000000005',
    'a1b2c3d4-2222-4000-8000-000000000002',
    'd0000002-0000-4000-8000-000000000002',
    'Graduate Associate Programme 2025 - Assurance',
    2025,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "CV, transcripts, and ICAN foundation certificate upload.", "tips": "If you have any ICAN exams completed, flag them prominently — it weighs heavily."},
        {"stage_name": "Korn Ferry Test", "description": "Numerical, verbal, and logical reasoning. 60 minutes total. Tougher than SHL.", "tips": "Focus on logical reasoning practice specifically — this is where most people struggle."},
        {"stage_name": "Group Exercise", "description": "Six candidates discussing a business case. Two partners observing quietly.", "tips": "Structure your contributions. Don''t dominate but don''t stay silent either."},
        {"stage_name": "Partner Interview", "description": "One-on-one with an assurance partner. Mix of technical accounting and behavioural questions.", "tips": "Read a recent audit news story before going in. I got asked about the Oando audit saga."}
    ]'::jsonb,
    5,
    4,
    E'PwC is rigorous but the feedback loop is genuinely helpful. The partner interview felt like a conversation, not an interrogation. Have a view on recent audit controversies and know your IFRS basics cold.',
    'Accounting',
    'University of Ibadan',
    'approved',
    '2025-06-10 13:20:00+01'
),
(
    'e0000002-0000-4000-8000-000000000006',
    'a1b2c3d4-2222-4000-8000-000000000002',
    'd0000001-0000-4000-8000-000000000001',
    'Graduate Associate Programme 2024 - Advisory',
    2024,
    'rejected',
    '[
        {"stage_name": "Online Application", "description": "Applied to the Advisory track rather than Assurance. Different process.", "tips": "Advisory is more competitive — make sure your CV shows analytical project work."},
        {"stage_name": "Korn Ferry Test", "description": "Passed the test comfortably.", "tips": "Don''t underestimate logical reasoning. It appears in every stage."},
        {"stage_name": "Group Exercise", "description": "This is where I struggled. I let more aggressive candidates dominate.", "tips": "Find your entry point within the first 5 minutes. Waiting makes it harder to join later."}
    ]'::jsonb,
    4,
    3,
    E'The group exercise is where most Advisory candidates get filtered out. Practice with friends beforehand if you can — timing when to speak is a learnable skill.',
    'Economics',
    'University of Lagos',
    'approved',
    '2024-05-30 16:00:00+01'
),
(
    'e0000002-0000-4000-8000-000000000007',
    'a1b2c3d4-2222-4000-8000-000000000002',
    'd0000006-0000-4000-8000-000000000006',
    'Graduate Associate Programme 2023 - Assurance',
    2023,
    'withdrew',
    '[
        {"stage_name": "Online Application", "description": "Completed the application and passed the Korn Ferry test.", "tips": "Start ICAN/ACCA studies before applying — it shows commitment to the field."},
        {"stage_name": "Group Exercise", "description": "Attended and did well.", "tips": "N/A"},
        {"stage_name": "Partner Interview", "description": "Scheduled but I withdrew before the interview — accepted a role at a bank instead.", "tips": "Be sure about sector before committing. Audit vs banking is a real choice to make."}
    ]'::jsonb,
    4,
    4,
    E'I withdrew for a banking role, not because of the process. PwC''s process was fair and I''d apply again if I had to choose today. Timing matters — many candidates end up choosing between multiple firms.',
    'Finance',
    'Covenant University',
    'approved',
    '2023-04-18 10:30:00+01'
),

-- ============== Shell — 2 reviews ==============
(
    'e0000003-0000-4000-8000-000000000008',
    'a1b2c3d4-4444-4000-8000-000000000004',
    'd0000003-0000-4000-8000-000000000003',
    'Shell Graduate Programme 2024',
    2024,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "CV and a Shell-specific questionnaire about values alignment.", "tips": "Actually read the Shell values. They genuinely filter on this."},
        {"stage_name": "Shell Online Assessment", "description": "Cognitive ability, situational judgement, and values assessment. Took about 90 minutes total.", "tips": "The situational judgement test rewards honesty over strategic answers. Answer what you''d actually do."},
        {"stage_name": "Video Interview", "description": "Six competency questions mapped to Shell values. Pre-recorded via HireVue.", "tips": "Map each answer explicitly to a Shell value. The structure helps assessors rate you fairly."},
        {"stage_name": "Assessment Day", "description": "Group exercise on energy transition, individual case study, technical interview with a senior engineer.", "tips": "The case study is specific to your discipline. Mine was reservoir engineering — know your fundamentals."}
    ]'::jsonb,
    5,
    5,
    E'Shell''s process is the longest and most thorough I went through. 12 weeks from application to offer. The assessment day felt like a proper engineering interview, not a generic competency check. Worth the effort.',
    'Mechanical Engineering',
    'Ahmadu Bello University',
    'approved',
    '2024-10-22 15:00:00+01'
),
(
    'e0000003-0000-4000-8000-000000000009',
    'a1b2c3d4-4444-4000-8000-000000000004',
    'd0000004-0000-4000-8000-000000000004',
    'Shell Graduate Programme 2025',
    2025,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "Smooth application process. The values questionnaire takes time — don''t rush it.", "tips": "Draft your values answers in a document first. You''ll want to revise them."},
        {"stage_name": "Shell Online Assessment", "description": "Well-designed test. The SJT was the most interesting part.", "tips": "Don''t overthink the SJT. Pick the answer that reflects what you''d genuinely do."},
        {"stage_name": "Video Interview", "description": "Felt more natural than Access Bank''s HireVue — the questions were deeper.", "tips": "Have a ''why Shell, why now'' answer that references the energy transition."},
        {"stage_name": "Assessment Day", "description": "Had a technical interview on subsurface engineering. Felt fair and thorough.", "tips": "Brush up on your discipline fundamentals — this is not a generic interview."}
    ]'::jsonb,
    5,
    5,
    E'If you can handle the length of Shell''s process, the quality of assessment is the best I encountered in Nigeria. They genuinely care about finding the right fit, not just rejecting people.',
    'Petroleum Engineering',
    'University of Port Harcourt',
    'approved',
    '2025-09-15 14:30:00+01'
),

-- ============== Flutterwave — 2 reviews ==============
(
    'e0000004-0000-4000-8000-000000000010',
    'a1b2c3d4-5555-4000-8000-000000000005',
    'd0000005-0000-4000-8000-000000000005',
    'Graduate Software Engineer 2025',
    2025,
    'offer',
    '[
        {"stage_name": "Online Application", "description": "CV and GitHub link. The recruiter actually browsed my repos.", "tips": "Pin your best 3-4 repos on GitHub. Quality matters more than quantity."},
        {"stage_name": "Take-Home Assignment", "description": "Build a small payments API in Go or Python. 48 hours to complete.", "tips": "Write tests. They explicitly evaluate test coverage, not just whether the code works."},
        {"stage_name": "Technical Interview", "description": "Live coding on a real problem, then system design discussion. 90 minutes total.", "tips": "The system design round is deep — they want you to reason about scale, not just draw boxes."},
        {"stage_name": "Culture Fit", "description": "Conversation with the hiring manager about ownership and shipping. Felt like a normal chat.", "tips": "Have concrete examples of features you shipped end-to-end, even if only for coursework."}
    ]'::jsonb,
    4,
    5,
    E'Flutterwave''s process is the fastest of any Nigerian company I applied to — offer came within 3 weeks. They care about shipping ability over pedigree. Make your GitHub look like a real engineer''s, not a student''s.',
    'Computer Science',
    'Obafemi Awolowo University',
    'approved',
    '2025-11-10 10:00:00+01'
),
(
    'e0000004-0000-4000-8000-000000000011',
    'a1b2c3d4-5555-4000-8000-000000000005',
    'd0000001-0000-4000-8000-000000000001',
    'Graduate Software Engineer 2024',
    2024,
    'rejected',
    '[
        {"stage_name": "Online Application", "description": "Applied with a strong CV but a sparse GitHub.", "tips": "Don''t skip the GitHub step. It''s their primary signal for engineering candidates."},
        {"stage_name": "Take-Home Assignment", "description": "Completed the API but didn''t write tests. That cost me.", "tips": "Tests are not optional. If you don''t know how to write them, learn before applying."},
        {"stage_name": "Technical Interview", "description": "Did okay on live coding but stumbled on system design — didn''t have enough experience.", "tips": "Read ''Designing Data-Intensive Applications'' chapter summaries before the interview."}
    ]'::jsonb,
    5,
    3,
    E'Flutterwave expects you to already be an engineer, not a fresh graduate. If you don''t have real projects and don''t know how to write tests, build those first before applying. Coming back stronger next cycle.',
    'Economics',
    'University of Lagos',
    'approved',
    '2024-12-01 16:45:00+01'
);

-- ============================================================================
-- Recalculate employer aggregate ratings from the seeded reviews.
-- Mirrors what ReviewModel.RecalculateRatings does in Go, but run once here
-- for all 5 employers. Flutterwave gets non-zero ratings now; Dangote stays null
-- since it has no reviews.
-- ============================================================================

UPDATE employer e
SET avg_difficulty_rating = sub.avg_diff,
    avg_experience_rating = sub.avg_exp,
    review_count          = sub.cnt
FROM (
    SELECT
        employer_id,
        AVG(difficulty_rating)::numeric(3,2) AS avg_diff,
        AVG(experience_rating)::numeric(3,2) AS avg_exp,
        COUNT(*)                             AS cnt
    FROM review
    WHERE status = 'approved'
    GROUP BY employer_id
) AS sub
WHERE e.id = sub.employer_id;