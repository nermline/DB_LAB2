-- Requirement 4: keys and indexes, created after the bulk load.
-- Requirement 5: every query of the benchmark has an index on both sides (PostgreSQL and MongoDB).

ALTER TABLE faculties      ADD CONSTRAINT uq_faculties_code UNIQUE (code);
ALTER TABLE departments    ADD CONSTRAINT uq_departments_code UNIQUE (code),
                           ADD CONSTRAINT fk_departments_faculty FOREIGN KEY (faculty_id) REFERENCES faculties (id);
ALTER TABLE specialties    ADD CONSTRAINT fk_specialties_department FOREIGN KEY (department_id) REFERENCES departments (id);
ALTER TABLE teachers       ADD CONSTRAINT fk_teachers_department FOREIGN KEY (department_id) REFERENCES departments (id);
ALTER TABLE student_groups ADD CONSTRAINT uq_groups_code UNIQUE (code),
                           ADD CONSTRAINT fk_groups_specialty FOREIGN KEY (specialty_id) REFERENCES specialties (id),
                           ADD CONSTRAINT fk_groups_curator FOREIGN KEY (curator_teacher_id) REFERENCES teachers (id);
ALTER TABLE students       ADD CONSTRAINT uq_students_record_book UNIQUE (record_book_no),
                           ADD CONSTRAINT fk_students_group FOREIGN KEY (group_id) REFERENCES student_groups (id);
ALTER TABLE semesters      ADD CONSTRAINT uq_semesters UNIQUE (academic_year, term);
ALTER TABLE courses        ADD CONSTRAINT uq_courses_code UNIQUE (code),
                           ADD CONSTRAINT fk_courses_department FOREIGN KEY (department_id) REFERENCES departments (id);
ALTER TABLE enrollments    ADD CONSTRAINT uq_enrollment UNIQUE (student_id, course_id, semester_id),
                           ADD CONSTRAINT fk_enrollments_student FOREIGN KEY (student_id) REFERENCES students (id),
                           ADD CONSTRAINT fk_enrollments_course FOREIGN KEY (course_id) REFERENCES courses (id),
                           ADD CONSTRAINT fk_enrollments_semester FOREIGN KEY (semester_id) REFERENCES semesters (id);
ALTER TABLE grades         ADD CONSTRAINT fk_grades_enrollment FOREIGN KEY (enrollment_id) REFERENCES enrollments (id),
                           ADD CONSTRAINT fk_grades_teacher FOREIGN KEY (teacher_id) REFERENCES teachers (id),
                           ADD CONSTRAINT chk_grades_score CHECK (score >= 0 AND score <= max_score);

CREATE INDEX ix_students_group               ON students (group_id);
CREATE INDEX ix_enrollments_semester_course  ON enrollments (semester_id, course_id) INCLUDE (status, total_score);
CREATE INDEX ix_enrollments_failed           ON enrollments (student_id) WHERE status = 'failed';
CREATE INDEX ix_grades_enrollment            ON grades (enrollment_id);
CREATE INDEX ix_student_records_group        ON student_records ((doc -> 'group' ->> 'code'));

DO $$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['faculties', 'departments', 'specialties', 'teachers', 'student_groups',
                             'students', 'semesters', 'courses', 'enrollments', 'grades']
    LOOP
        EXECUTE format('SELECT setval(pg_get_serial_sequence(%L, ''id''), (SELECT COALESCE(max(id), 0) + 1 FROM %I), false)', t, t);
    END LOOP;
END;
$$;

ANALYZE;
