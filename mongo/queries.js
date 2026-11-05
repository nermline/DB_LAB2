// Requirement 3: MongoDB queries of the benchmark, for manual runs:
//   mongosh university_lab2 mongo/queries.js
const records = db.student_records;
const show = (title, value) => { print(`\n=== ${title}`); printjson(value); };
const stages = (p) => (p ? [p.stage + (p.indexName ? `(${p.indexName})` : "")].concat(stages(p.inputStage)) : []);
const plan = (title, explain) => {
  const q = explain.queryPlanner || explain.stages[0].$cursor.queryPlanner;
  const winning = q.winningPlan.queryPlan || q.winningPlan;
  print(`\n--- plan ${title}: ${stages(winning).join(" <- ")}`);
};

// B1: the whole student record by id
const rec = records.findOne({ _id: 1 });
show("B1 student record (shortened)", {
  _id: rec._id, name: `${rec.last_name} ${rec.first_name}`, group: rec.group.code,
  semesters: rec.semesters.length,
  first_course: { ...rec.semesters[0].courses[0], grades: rec.semesters[0].courses[0].grades.slice(0, 2) },
});
plan("B1", records.find({ _id: 1 }).explain());

// B2: students of a group with failed courses
const debtors = [
  { $match: { "group.code": "SE-24-01", "semesters.courses.status": "failed" } },
  { $project: { last_name: 1, first_name: 1, failed: { $sum: { $map: { input: "$semesters", as: "s",
      in: { $size: { $filter: { input: "$$s.courses", as: "c", cond: { $eq: ["$$c.status", "failed"] } } } } } } } } },
  { $sort: { _id: 1 } },
];
show("B2 group debtors", records.aggregate(debtors).toArray());
plan("B2", records.explain().aggregate(debtors));

// B3: average score and pass rate of every course in a semester
const stats = [
  { $match: { "semesters.semester_id": 8 } },
  { $unwind: "$semesters" },
  { $match: { "semesters.semester_id": 8 } },
  { $unwind: "$semesters.courses" },
  { $match: { "semesters.courses.status": { $in: ["completed", "failed"] } } },
  { $group: { _id: "$semesters.courses.code", name: { $first: "$semesters.courses.name" }, students: { $sum: 1 },
              avg: { $avg: "$semesters.courses.total_score" },
              passed: { $sum: { $cond: [{ $eq: ["$semesters.courses.status", "completed"] }, 1, 0] } } } },
  { $sort: { _id: 1 } },
  { $limit: 5 },
];
show("B3 course statistics (first 5 courses)", records.aggregate(stats).toArray());
plan("B3", records.explain().aggregate(stats));

// B4: add a grade to an embedded course (and remove it again)
const cur = records.findOne({ _id: 1 }, { semesters: { $slice: -1 } });
const sem = cur.semesters[0];
const enrollment = sem.courses[0].enrollment_id;
show("B4 add grade", records.updateOne(
  { _id: 1 },
  { $push: { "semesters.$[s].courses.$[c].grades": { grade_id: -1, type: "bonus", title: "Demo bonus", score: 1,
             max_score: 5, graded_on: new Date(), teacher_id: 1, teacher: "demo" } } },
  { arrayFilters: [{ "s.semester_id": sem.semester_id }, { "c.enrollment_id": enrollment }], writeConcern: { j: true } }));
records.updateOne({ _id: 1 }, { $pull: { "semesters.$[].courses.$[].grades": { title: "Demo bonus" } } });

// B5: rename a course in every document that contains it (and rename it back)
const course = rec.semesters[0].courses[0];
show(`B5 rename course ${course.code}`, records.updateMany(
  { "semesters.courses.course_id": course.course_id },
  { $set: { "semesters.$[].courses.$[c].name": course.name + " (renamed)" } },
  { arrayFilters: [{ "c.course_id": course.course_id }] }));
records.updateMany(
  { "semesters.courses.course_id": course.course_id },
  { $set: { "semesters.$[].courses.$[c].name": course.name } },
  { arrayFilters: [{ "c.course_id": course.course_id }] });

show("indexes", records.getIndexes().map(i => i.key));
