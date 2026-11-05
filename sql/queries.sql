-- Requirement 4: PostgreSQL queries of the benchmark with their plans, for manual runs:
--   psql -d university_lab2 -f sql/queries.sql
\pset pager off

\echo '\n=== B1 student record: header (student + group + specialty + department + faculty)'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT s.id, s.record_book_no, s.last_name, s.first_name, g.code, sp.name, d.name, f.name
FROM students s
JOIN student_groups g ON g.id  = s.group_id
JOIN specialties sp   ON sp.id = g.specialty_id
JOIN departments d    ON d.id  = sp.department_id
JOIN faculties f      ON f.id  = d.faculty_id
WHERE s.id = 1;

\echo '\n=== B1 student record: body (enrollments + semesters + courses + grades + teachers)'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT e.semester_id, sm.academic_year, c.code, c.name, e.status, e.total_score,
       gr.title, gr.score, gr.max_score, t.last_name
FROM enrollments e
JOIN semesters sm    ON sm.id = e.semester_id
JOIN courses c       ON c.id  = e.course_id
LEFT JOIN grades gr  ON gr.enrollment_id = e.id
LEFT JOIN teachers t ON t.id  = gr.teacher_id
WHERE e.student_id = 1
ORDER BY e.semester_id, c.code, gr.graded_on, gr.id;

\echo '\n=== B1 student record: jsonb document'
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT doc FROM student_records WHERE id = 1;

\echo '\n=== B2 group debtors'
SELECT s.id, s.last_name, s.first_name, count(*) AS failed
FROM student_groups g
JOIN students s    ON s.group_id = g.id
JOIN enrollments e ON e.student_id = s.id AND e.status = 'failed'
WHERE g.code = 'SE-24-01'
GROUP BY s.id
ORDER BY s.id;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT s.id, count(*)
FROM student_groups g
JOIN students s    ON s.group_id = g.id
JOIN enrollments e ON e.student_id = s.id AND e.status = 'failed'
WHERE g.code = 'SE-24-01'
GROUP BY s.id;

\echo '\n=== B3 course statistics of semester 8 (first 5 courses)'
SELECT c.code, c.name, count(*) AS students, round(avg(e.total_score), 2) AS avg,
       count(*) FILTER (WHERE e.status = 'completed') AS passed
FROM enrollments e
JOIN courses c ON c.id = e.course_id
WHERE e.semester_id = 8 AND e.status IN ('completed', 'failed')
GROUP BY c.code, c.name
ORDER BY c.code
LIMIT 5;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
SELECT c.code, count(*), avg(e.total_score)
FROM enrollments e
JOIN courses c ON c.id = e.course_id
WHERE e.semester_id = 8 AND e.status IN ('completed', 'failed')
GROUP BY c.code;

\echo '\n=== B4 add grade, B5 rename course (rolled back)'
BEGIN;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
INSERT INTO grades (enrollment_id, assessment_type, title, score, max_score, graded_on, teacher_id)
SELECT id, 'bonus', 'Demo bonus', 1, 5, CURRENT_DATE, 1 FROM enrollments WHERE student_id = 1 ORDER BY id DESC LIMIT 1;
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
UPDATE courses SET name = name || ' (renamed)' WHERE id = 21;
ROLLBACK;
