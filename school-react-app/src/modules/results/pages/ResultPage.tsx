import { AppIcon } from "shared/ui/AppIcon";
import { useMemo, useState, useCallback, useEffect, useRef } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { Card, DataState, Skeleton, TableSkeleton, Badge, DataTable, DataTableColumn, RowAction, StatCardGrid } from "@/components/ui";
import { useSafeAsync } from "@/hooks/useSafeAsync";
import { serviceRequest } from "@/services/service-client";
import { ResultForm } from "../components/ResultForm";
import { useResults } from "../hooks/useResults";
import { ResultRow } from "../types/result.types";
import { useClasses } from "../../classes/hooks/useClasses";
import { useExams } from "../../exams/hooks/useExams";
import { useQueryParams } from "@/hooks/useQueryParams";
import { exportMarksheet, exportExamMarksheet } from "@/utils/marksheet";
import { showToast } from "@/utils/toast";
import { useSchoolBranding } from "@/hooks/useSchoolBranding";
import { useCertificateTemplates } from "@/modules/certificates/hooks/useCertificates";
import { BulkGeneratorModal } from "@/modules/certificates/components/BulkGeneratorModal";
import type { CertificateTemplate } from "@/modules/certificates/types/certificate.types";

// ─── Rank badge helper ─────────────────────────────────────────────────────
function RankBadge({ rank }: { rank: number }) {
  if (rank === 1)
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-amber-50 border border-amber-200 text-amber-700 text-[11px] font-black">
        <span>🥇</span> 1st
      </span>
    );
  if (rank === 2)
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-slate-50 border border-slate-200 text-slate-600 text-[11px] font-black">
        <span>🥈</span> 2nd
      </span>
    );
  if (rank === 3)
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-orange-50 border border-orange-200 text-orange-700 text-[11px] font-black">
        <span>🥉</span> 3rd
      </span>
    );
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-full bg-slate-50 border border-slate-100 text-slate-500 text-[11px] font-bold">
      #{rank}
    </span>
  );
}

// ─── Grade helper ─────────────────────────────────────────────────────────
function getGradeColor(grade: string) {
  if (grade === "A+" || grade === "A") return "bg-emerald-50 text-emerald-700 border-emerald-200";
  if (grade === "B") return "bg-blue-50 text-blue-700 border-blue-200";
  if (grade === "C") return "bg-amber-50 text-amber-700 border-amber-200";
  if (grade === "D") return "bg-orange-50 text-orange-700 border-orange-200";
  return "bg-rose-50 text-rose-700 border-rose-200";
}

// ─── Class Result Sheet ────────────────────────────────────────────────────
function ClassResultSheet({
  rows,
  examTitle,
  schoolName,
  logoUrl,
  onExportRow,
}: {
  rows: ResultRow[];
  examTitle: string;
  schoolName: string;
  logoUrl?: string;
  onExportRow: (row: ResultRow) => void;
}) {
  const printRef = useRef<HTMLDivElement>(null);

  // Gather all unique subjects across all rows
  const allSubjects = useMemo(() => {
    const map = new Map<string, string>(); // id -> name
    for (const row of rows) {
      for (const sub of row.subjects || []) {
        if (sub.subject_id && !map.has(sub.subject_id)) {
          map.set(sub.subject_id, sub.subject_name);
        }
      }
    }
    // If no subjects breakdown, use exam_subject split
    if (map.size === 0) {
      rows.forEach((row) => {
        (row.exam_subject || "").split(",").forEach((s) => {
          const t = s.trim();
          if (t) map.set(t, t);
        });
      });
    }
    return Array.from(map.entries()).map(([id, name]) => ({ id, name }));
  }, [rows]);

  // Rank rows by obtained_marks desc within the same group
  const rankedRows = useMemo(() => {
    const sorted = [...rows].sort((a, b) => b.obtained_marks - a.obtained_marks);
    return sorted.map((row, idx) => ({ ...row, _rank: idx + 1 }));
  }, [rows]);

  const handlePrint = () => {
    const el = printRef.current;
    if (!el) return;
    const printWin = window.open("", "_blank");
    if (!printWin) return;
    printWin.document.write(`
      <html>
        <head>
          <title>${examTitle} – Class Result Sheet</title>
          <style>
            * { box-sizing: border-box; margin: 0; padding: 0; }
            body { font-family: Arial, sans-serif; font-size: 11px; padding: 20px; background: white; }
            h1 { font-size: 16px; font-weight: 900; margin-bottom: 4px; }
            p { font-size: 11px; color: #666; margin-bottom: 12px; }
            table { width: 100%; border-collapse: collapse; }
            th, td { border: 1px solid #ddd; padding: 6px 10px; text-align: left; white-space: nowrap; }
            th { background: #f8fafc; font-weight: 700; font-size: 10px; text-transform: uppercase; letter-spacing: 0.05em; }
            tr:nth-child(even) td { background: #fafafa; }
            .rank-1 td { background: #fffbeb !important; }
            .rank-2 td { background: #f8fafc !important; }
            .rank-3 td { background: #fff7ed !important; }
            .center { text-align: center; }
            @media print { body { padding: 10px; } }
          </style>
        </head>
        <body>${el.innerHTML}</body>
      </html>
    `);
    printWin.document.close();
    printWin.focus();
    printWin.print();
  };

  if (rows.length === 0) {
    return (
      <DataState
        variant="empty"
        title="No results for this selection"
        message="Select a class and exam to see the result sheet."
      />
    );
  }

  return (
    <div className="space-y-4">
      {/* Actions bar */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h2 className="text-sm font-black text-slate-900">{examTitle}</h2>
          <p className="text-[11px] text-slate-400 font-medium">
            {rankedRows.length} students · {allSubjects.length} subjects
          </p>
        </div>
        <button
          onClick={handlePrint}
          className="h-9 px-5 rounded-xl bg-slate-900 text-white text-[10px] font-black uppercase tracking-widest flex items-center gap-2 hover:bg-slate-800 transition-all active:scale-95 shadow-sm"
        >
          <AppIcon name="Printer" size={16} />
          Print Result Sheet
        </button>
      </div>

      {/* Table */}
      <div ref={printRef} className="overflow-x-auto rounded-2xl border border-slate-200 shadow-sm bg-white">
        {/* Print header (only visible when printing) */}
        <div className="hidden print:block px-6 pt-6 pb-4 border-b border-slate-100">
          <h1 className="text-lg font-black text-slate-900">{schoolName} – Result Sheet</h1>
          <p className="text-xs text-slate-500">{examTitle} · Printed {new Date().toLocaleDateString()}</p>
        </div>

        <table className="w-full text-left border-collapse min-w-[900px]">
          <thead>
            <tr className="bg-slate-50 border-b border-slate-200">
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-400 sticky left-0 bg-slate-50 z-10 whitespace-nowrap">#</th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-400 sticky left-8 bg-slate-50 z-10 whitespace-nowrap min-w-[160px]">Student</th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-400 whitespace-nowrap">Adm. No</th>
              {allSubjects.map((sub) => (
                <th
                  key={sub.id}
                  className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-blue-500 text-center whitespace-nowrap"
                >
                  {sub.name.length > 10 ? sub.name.slice(0, 9) + "…" : sub.name}
                </th>
              ))}
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-700 text-center whitespace-nowrap bg-slate-100 border-l border-slate-200">
                Total
              </th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-500 text-center whitespace-nowrap">%</th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-500 text-center whitespace-nowrap">Grade</th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-amber-500 text-center whitespace-nowrap">Position</th>
              <th className="px-3 py-3.5 text-[9px] font-black uppercase tracking-widest text-slate-400 text-center whitespace-nowrap no-print">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {rankedRows.map((row, idx) => {
              const pct = row.max_marks > 0 ? Math.round((row.obtained_marks / row.max_marks) * 100) : 0;
              const isTopThree = row._rank <= 3;
              const rowBg = row._rank === 1
                ? "bg-amber-50/60"
                : row._rank === 2
                ? "bg-slate-50/80"
                : row._rank === 3
                ? "bg-orange-50/40"
                : "bg-white hover:bg-slate-50/50";

              return (
                <tr
                  key={row._id}
                  className={`transition-colors ${rowBg} ${isTopThree ? "rank-" + row._rank : ""}`}
                >
                  {/* Rank number */}
                  <td className="px-3 py-3 text-[11px] font-bold text-slate-400 sticky left-0 z-10 whitespace-nowrap"
                    style={{ background: "inherit" }}>
                    {idx + 1}
                  </td>

                  {/* Student name */}
                  <td className="px-3 py-3 sticky left-8 z-10 whitespace-nowrap min-w-[160px]"
                    style={{ background: "inherit" }}>
                    <div className="flex items-center gap-2">
                      <div className={`h-7 w-7 rounded-lg flex items-center justify-center text-[10px] font-black text-white shadow-sm shrink-0 ${
                        row._rank === 1 ? "bg-amber-500" : row._rank === 2 ? "bg-slate-400" : row._rank === 3 ? "bg-orange-400" : "bg-slate-700"
                      }`}>
                        {(row.student_name || "S")[0]}
                      </div>
                      <div>
                        <p className="text-[11px] font-bold text-slate-900 leading-none">{row.student_name}</p>
                        <p className="text-[9px] text-slate-400 font-medium mt-0.5">{row.class_name}</p>
                      </div>
                    </div>
                  </td>

                  {/* Admission No */}
                  <td className="px-3 py-3 text-[10px] font-bold text-slate-400 whitespace-nowrap">
                    {row.admission_no}
                  </td>

                  {/* Per-subject marks */}
                  {allSubjects.map((sub) => {
                    const subRecord = (row.subjects || []).find((s) => s.subject_id === sub.id || s.subject_name === sub.name);
                    const isAbsent = subRecord?.obtained_marks === -1;
                    const hasMarks = subRecord?.obtained_marks !== undefined;
                    const subPct = hasMarks && !isAbsent && subRecord!.max_marks > 0
                      ? (subRecord!.obtained_marks / subRecord!.max_marks) * 100
                      : 0;
                    return (
                      <td key={sub.id} className="px-3 py-3 text-center whitespace-nowrap">
                        {hasMarks ? (
                          <span
                            className={`inline-block text-[11px] font-black px-2 py-0.5 rounded-lg border ${
                              isAbsent
                                ? "bg-rose-50 border-rose-100 text-rose-500"
                                : subPct >= 80
                                ? "bg-emerald-50 border-emerald-100 text-emerald-700"
                                : subPct >= 40
                                ? "bg-blue-50 border-blue-100 text-blue-700"
                                : "bg-rose-50 border-rose-100 text-rose-600"
                            }`}
                          >
                            {isAbsent ? "AB" : subRecord!.obtained_marks}
                            <span className="text-[8px] font-medium text-current/50 ml-0.5">
                              /{subRecord!.max_marks}
                            </span>
                          </span>
                        ) : (
                          <span className="text-[10px] text-slate-300 font-bold">—</span>
                        )}
                      </td>
                    );
                  })}

                  {/* Total */}
                  <td className="px-3 py-3 text-center whitespace-nowrap bg-slate-50/80 border-l border-slate-200">
                    <span className="text-[12px] font-black text-slate-900">
                      {row.obtained_marks}
                      <span className="text-[9px] font-medium text-slate-400">/{row.max_marks}</span>
                    </span>
                  </td>

                  {/* Percentage */}
                  <td className="px-3 py-3 text-center whitespace-nowrap">
                    <span className={`text-[11px] font-black ${pct >= 80 ? "text-emerald-600" : pct >= 40 ? "text-blue-600" : "text-rose-600"}`}>
                      {pct}%
                    </span>
                  </td>

                  {/* Grade */}
                  <td className="px-3 py-3 text-center whitespace-nowrap">
                    <span className={`inline-block text-[10px] font-black px-2 py-0.5 rounded-md border ${getGradeColor(row.grade)}`}>
                      {row.grade}
                    </span>
                  </td>

                  {/* Position */}
                  <td className="px-3 py-3 text-center whitespace-nowrap">
                    <RankBadge rank={row._rank} />
                  </td>

                  {/* Actions */}
                  <td className="px-3 py-3 text-center whitespace-nowrap no-print">
                    <button
                      onClick={() => onExportRow(row)}
                      title="Download marksheet"
                      className="h-7 w-7 rounded-lg bg-blue-50 hover:bg-blue-600 text-blue-600 hover:text-white border border-blue-100 hover:border-blue-600 flex items-center justify-center transition-all mx-auto"
                    >
                      <AppIcon name="Download" size={13} />
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
          {/* Footer summary */}
          <tfoot>
            <tr className="bg-slate-100 border-t-2 border-slate-200">
              <td colSpan={3} className="px-3 py-3 text-[10px] font-black uppercase tracking-widest text-slate-500">
                Class Average
              </td>
              {allSubjects.map((sub) => {
                const vals = rankedRows
                  .map((r) => (r.subjects || []).find((s) => s.subject_id === sub.id || s.subject_name === sub.name)?.obtained_marks)
                  .filter((v): v is number => v !== undefined && v !== -1);
                const avg = vals.length > 0 ? Math.round(vals.reduce((a, b) => a + b, 0) / vals.length) : null;
                return (
                  <td key={sub.id} className="px-3 py-3 text-center text-[10px] font-black text-slate-600">
                    {avg !== null ? avg : "—"}
                  </td>
                );
              })}
              <td className="px-3 py-3 text-center bg-slate-200 border-l border-slate-300">
                <span className="text-[11px] font-black text-slate-700">
                  {rankedRows.length > 0
                    ? Math.round(rankedRows.reduce((a, r) => a + r.obtained_marks, 0) / rankedRows.length)
                    : "—"}
                </span>
              </td>
              <td className="px-3 py-3 text-center text-[11px] font-black text-slate-600">
                {rankedRows.length > 0
                  ? Math.round(rankedRows.reduce((a, r) => a + (r.obtained_marks / r.max_marks) * 100, 0) / rankedRows.length)
                  : "—"}%
              </td>
              <td colSpan={3} />
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  );
}

// ─── Main Page ────────────────────────────────────────────────────────────
export function ResultPage() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { state: templatesState } = useCertificateTemplates();
  const [selectedTemplate, setSelectedTemplate] = useState<CertificateTemplate | null>(null);

  const resultTemplates = (templatesState.data || []).filter(
    (t) => (t.type as string) === "result_card"
  );
  const isTeacher = pathname.includes("/teacher");
  const { currentParams, updateQuery } = useQueryParams();
  const exam_id = currentParams.get("exam_id") || "all";
  const class_id = currentParams.get("class_id") || "all";
  const student_id = currentParams.get("student_id") || "all";
  const subject_id = currentParams.get("subject_id") || "all";
  const date_filter = currentParams.get("date") || "";

  const { schoolName, logoUrl } = useSchoolBranding();
  const brandedSchoolName = schoolName || "School";

  const { state: classState } = useClasses();
  const { state: examListState } = useExams(class_id !== "all" ? { class_id } : {});

  const { state, addResult } = useResults({
    exam_id: exam_id !== "all" ? exam_id : undefined,
    class_id: class_id !== "all" ? class_id : undefined,
    student_id: student_id !== "all" ? student_id : undefined,
    subject_id: subject_id !== "all" ? subject_id : undefined,
  });
  const [searchQuery, setSearchQuery] = useState(currentParams.get("search") || "");
  // "grid" | "list" | "sheet"
  const [viewMode, setViewMode] = useState<"grid" | "list" | "sheet">("list");
  const [isAdding, setIsAdding] = useState(false);

  const [classFilter, setClassFilter] = useState(class_id);
  const [examFilter, setExamFilter] = useState(exam_id);
  const [studentFilter, setStudentFilter] = useState(student_id);
  const [subjectFilter, setSubjectFilter] = useState(subject_id);
  const [dateFilter, setDateFilter] = useState(date_filter);

  useEffect(() => {
    setSearchQuery(currentParams.get("search") || "");
    setClassFilter(currentParams.get("class_id") || "all");
    setExamFilter(currentParams.get("exam_id") || "all");
    setStudentFilter(currentParams.get("student_id") || "all");
    setSubjectFilter(currentParams.get("subject_id") || "all");
    setDateFilter(currentParams.get("date") || "");
  }, [currentParams.toString()]);

  const { state: examState, run: runExams } = useSafeAsync<
    Array<{ _id: string; title: string; subject: string; class_name?: string; class_id?: string }>
  >();
  const { state: studentState, run: runStudents } = useSafeAsync<
    Array<{ _id: string; first_name: string; last_name: string; admission_no: string; class_id: string }>
  >();

  const loadExams = useCallback(() => {
    return runExams(async () => {
      const result = await serviceRequest<
        Array<{ _id: string; title: string; subject: string; class_name?: string; class_id?: string }>
      >("/api/exams");
      if (!result.ok) throw new Error(result.error.message || "Failed to load exams");
      return result.data;
    });
  }, [runExams]);

  const loadStudents = useCallback(() => {
    return runStudents(async () => {
      const result = await serviceRequest<
        Array<{ _id: string; first_name: string; last_name: string; admission_no: string; class_id: string }>
      >("/api/students");
      if (!result.ok) throw new Error(result.error.message || "Failed to load students");
      return result.data;
    });
  }, [runStudents]);

  useEffect(() => {
    void loadExams().catch(() => {});
    void loadStudents().catch(() => {});
  }, [loadExams, loadStudents]);

  const filteredRows = useMemo(() => {
    const rows = state.data || [];
    const q = searchQuery.trim().toLowerCase();
    return rows.filter((row) => {
      const queryMatch =
        q.length === 0 ||
        row.student_name.toLowerCase().includes(q) ||
        row.exam_title.toLowerCase().includes(q) ||
        row.exam_subject.toLowerCase().includes(q) ||
        (row.admission_no || "").toLowerCase().includes(q) ||
        (row.class_name || "").toLowerCase().includes(q);

      const classMatch = classFilter === "all" ? true : row.class_id === classFilter;
      const studentMatch = studentFilter === "all" ? true : row.student_id === studentFilter;
      const subjectMatch =
        subjectFilter === "all"
          ? true
          : (row.subjects || []).some((s: any) => s.subject_id === subjectFilter);
      const dateMatch = dateFilter === "" ? true : row.graded_at?.split("T")[0] === dateFilter;

      return queryMatch && classMatch && studentMatch && subjectMatch && dateMatch;
    });
  }, [state.data, searchQuery, classFilter, studentFilter, subjectFilter, dateFilter]);

  const hasActiveFilters = Boolean(
    searchQuery.trim() ||
    classFilter !== "all" ||
    examFilter !== "all" ||
    studentFilter !== "all" ||
    subjectFilter !== "all" ||
    dateFilter
  );

  const clearAllFilters = useCallback(() => {
    setSearchQuery("");
    setClassFilter("all");
    setExamFilter("all");
    setStudentFilter("all");
    setSubjectFilter("all");
    setDateFilter("");
    updateQuery({
      search: "",
      class_id: "all",
      exam_id: "all",
      student_id: "all",
      subject_id: "all",
      date: "",
    });
  }, [updateQuery]);

  // For sheet view: group by exam if exam_id is "all", else single group
  const sheetExamTitle = useMemo(() => {
    if (examFilter !== "all") {
      const ex = (examListState.data || []).find((e: any) => e._id === examFilter);
      return ex ? (ex as any).title : "Selected Exam";
    }
    // multiple exams — use generic title
    return classFilter !== "all"
      ? `All Exams — ${((classState.data as any)?.data || []).find((c: any) => (c._id || c.id) === classFilter)?.name || "Class"}`
      : "All Results";
  }, [examFilter, classFilter, examListState.data, classState.data]);

  const columns: DataTableColumn<ResultRow>[] = [
    {
      key: "student",
      label: "Student performance",
      render: (row) => (
        <div className="flex items-center gap-3">
          <div className="h-8 w-8 rounded-lg bg-slate-900 text-white flex items-center justify-center text-[10px] font-bold normal-case">
            {(row.student_name || "S").substring(0, 1)}
          </div>
          <div>
            <p className="font-bold text-slate-900 leading-none mb-1">{row.student_name}</p>
            <p className="text-[10px] text-slate-400 font-bold normal-case tracking-tighter">{row.admission_no}</p>
          </div>
        </div>
      ),
    },
    {
      key: "exam",
      label: "Assessment",
      render: (row) => {
        const count =
          row.subjects && row.subjects.length > 0
            ? row.subjects.length
            : row.exam_subject
            ? row.exam_subject.split(",").filter((s) => s.trim()).length
            : 0;
        return (
          <div>
            <div className="flex items-center gap-1.5 mb-1 flex-wrap">
              <p className="text-[11px] font-bold text-slate-700 leading-none tracking-tight">{row.exam_title}</p>
              {row.exam_term && (
                <Badge variant="secondary" className="text-[8px] font-extrabold px-1 py-0">
                  {row.exam_term}
                </Badge>
              )}
            </div>
            <p className="text-[9px] font-bold text-blue-600 normal-case">
              {count} {count === 1 ? "subject" : "subjects"}
            </p>
          </div>
        );
      },
    },
    {
      key: "subject_breakdown",
      label: "Subjects",
      render: (row) => {
        const subjects = row.subjects || [];
        if (subjects.length === 0)
          return <span className="text-[10px] font-bold text-slate-300">—</span>;
        return (
          <div className="flex flex-wrap gap-1 max-w-[260px]">
            {subjects.map((s) => {
              const isAbsent = s.obtained_marks === -1;
              const pct = s.max_marks > 0 && !isAbsent ? (s.obtained_marks / s.max_marks) * 100 : 0;
              const tone = isAbsent
                ? "bg-rose-50 border-rose-100 text-rose-700"
                : pct >= 50
                ? "bg-emerald-50 border-emerald-100 text-emerald-700"
                : "bg-amber-50 border-amber-100 text-amber-700";
              return (
                <span
                  key={s.subject_id}
                  title={`${s.subject_name}: ${isAbsent ? "Absent" : `${s.obtained_marks} / ${s.max_marks}`}`}
                  className={`inline-flex items-center gap-1 h-5 px-1.5 rounded-md border text-[9px] font-bold ${tone}`}
                >
                  <span className="truncate max-w-[70px]">{s.subject_name}</span>
                  <span className="text-current/60">{isAbsent ? "AB" : `${s.obtained_marks}/${s.max_marks}`}</span>
                </span>
              );
            })}
          </div>
        );
      },
    },
    {
      key: "score",
      label: "Total",
      render: (row) => {
        const percentage = (row.obtained_marks / row.max_marks) * 100;
        return (
          <div className="flex flex-col w-32">
            <div className="flex items-center justify-between mb-1">
              <span className="text-[11px] font-bold text-slate-700">
                {row.obtained_marks} / {row.max_marks}
              </span>
              <span
                className={`text-[10px] font-bold ${
                  percentage >= 80 ? "text-emerald-600" : percentage >= 50 ? "text-blue-600" : "text-red-500"
                }`}
              >
                {percentage.toFixed(0)}%
              </span>
            </div>
            <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-500 rounded-full ${
                  percentage >= 80 ? "bg-emerald-500" : percentage >= 50 ? "bg-blue-500" : "bg-red-500"
                }`}
                style={{ width: `${percentage}%` }}
              />
            </div>
          </div>
        );
      },
    },
    {
      key: "grade",
      label: "Grade",
      render: (row) => (
        <Badge
          variant={row.grade === "A" || row.grade === "A+" ? "success" : row.grade === "F" ? "error" : "primary"}
          className="text-[10px] font-bold normal-case px-2 py-0.5"
        >
          {row.grade}
        </Badge>
      ),
    },
  ];

  const rowActions: RowAction<ResultRow>[] = [
    {
      icon: "visibility",
      label: "View",
      variant: "ghost",
      onClick: (row) => {
        const base = isTeacher ? "/teacher" : "/admin";
        navigate(`${base}/results/${row._id}`);
      },
    },
    {
      icon: "download",
      label: "Marksheet",
      variant: "primary",
      onClick: (row) => {
        exportMarksheet(row, { schoolName: brandedSchoolName, logoUrl });
        showToast("Generating marksheet…", "info");
      },
    },
  ];

  const isDependencyLoading =
    examState.status === "idle" ||
    examState.status === "loading" ||
    studentState.status === "idle" ||
    studentState.status === "loading";

  const examOptions = (examState.data ?? []).map((item) => ({
    id: item._id,
    class_id: item.class_id,
    label: `${item.title} - ${item.subject}`.trim(),
  }));

  const studentOptions = (studentState.data ?? []).map((item) => ({
    id: item._id,
    class_id: item.class_id,
    label: `${item.admission_no} - ${item.first_name} ${item.last_name}`.trim(),
  }));

  const subjectOptions = useMemo(() => {
    const seen = new Map<string, string>();
    (state.data || []).forEach((row) => {
      (row.subjects || []).forEach((subject: any) => {
        if (subject.subject_id && !seen.has(subject.subject_id)) {
          seen.set(subject.subject_id, subject.subject_name || subject.subject_id);
        }
      });
    });
    return Array.from(seen.entries()).map(([id, label]) => ({ id, label }));
  }, [state.data]);

  return (
    <div className="space-y-6 relative min-h-[80vh] pb-10">
      {/* Stats Section */}
      <StatCardGrid
        items={(() => {
          const rows = state.data || [];
          const totalResults = rows.length;
          const avgPerformance =
            totalResults > 0
              ? Math.round(rows.reduce((sum, r) => sum + (r.obtained_marks / r.max_marks) * 100, 0) / totalResults)
              : 0;
          const passCount = rows.filter((r) => r.obtained_marks / r.max_marks >= 0.4).length;
          const distinctionCount = rows.filter((r) => r.obtained_marks / r.max_marks >= 0.8).length;
          return [
            { label: "Total Results", value: totalResults, icon: "leaderboard", accent: "blue" as const },
            { label: "Avg. Score", value: `${avgPerformance}%`, icon: "trending_up", accent: "emerald" as const },
            { label: "Distinctions", value: distinctionCount, icon: "stars", accent: "amber" as const },
            {
              label: "Pass Rate",
              value: totalResults > 0 ? `${Math.round((passCount / totalResults) * 100)}%` : "0%",
              icon: "verified",
              accent: "purple" as const,
            },
          ];
        })()}
      />

      {/* Toolbar Section - Two-tiered, Clean & Fully Responsive */}
      <div className="premium-card p-3 sm:p-4 flex flex-col gap-3 bg-white/90 backdrop-blur-md border border-slate-200/80 shadow-sm rounded-2xl no-print">
        {/* Tier 1: Search bar + View Switcher + Print + Add Result */}
        <div className="flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
          {/* Search bar */}
          <div className="relative flex-1 min-w-[200px]">
            <AppIcon name="Search" size={16} className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              value={searchQuery}
              onChange={(e) => {
                const val = e.target.value;
                setSearchQuery(val);
                updateQuery({ search: val });
              }}
              placeholder="Search student, exam, admission no or class…"
              className="h-10 w-full rounded-xl border border-slate-200 bg-white/90 pl-10 pr-9 text-xs font-medium text-slate-700 outline-none transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-600/10 placeholder:text-slate-400"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => {
                  setSearchQuery("");
                  updateQuery({ search: "" });
                }}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 p-0.5 rounded-full transition-colors"
                title="Clear search"
              >
                <AppIcon name="X" size={14} />
              </button>
            )}
          </div>

          {/* Action buttons: Print, View toggle, Add Result */}
          <div className="flex items-center gap-2 justify-end flex-wrap sm:flex-nowrap">
            {/* Print / Export */}
            <button
              type="button"
              onClick={() => {
                exportExamMarksheet(filteredRows, { schoolName: brandedSchoolName, logoUrl });
                showToast("Generating printable report…", "info");
              }}
              className="h-10 px-3.5 rounded-xl border border-slate-200 bg-white text-[11px] font-bold text-slate-600 hover:text-blue-600 hover:border-blue-200 transition-all flex items-center gap-1.5 shadow-xs shrink-0 cursor-pointer"
              title="Print report"
            >
              <AppIcon name="Printer" size={15} />
              <span className="hidden sm:inline">Print</span>
            </button>

            <div className="hidden sm:block h-6 w-px bg-slate-200 shrink-0" />

            {/* View mode toggle */}
            <div className="flex items-center rounded-xl bg-slate-100 p-1 border border-slate-200/60 shrink-0 gap-0.5">
              {([
                { key: "list", icon: "List", label: "List" },
                { key: "grid", icon: "LayoutGrid", label: "Grid" },
                { key: "sheet", icon: "Grid", label: "Sheet" },
              ] as const).map((vm) => (
                <button
                  key={vm.key}
                  type="button"
                  onClick={() => setViewMode(vm.key)}
                  title={`${vm.label} view`}
                  className={`flex h-8 items-center gap-1.5 rounded-lg px-2.5 text-[11px] font-bold transition-all cursor-pointer ${
                    viewMode === vm.key
                      ? "bg-white text-blue-600 shadow-sm font-black"
                      : "text-slate-500 hover:text-slate-800"
                  }`}
                >
                  <AppIcon name={vm.icon} size={14} />
                  <span>{vm.label}</span>
                </button>
              ))}
            </div>

            <div className="hidden sm:block h-6 w-px bg-slate-200 shrink-0" />

            {/* Add Result */}
            <button
              type="button"
              onClick={() => setIsAdding(!isAdding)}
              className={`inline-flex h-10 items-center gap-2 px-4 text-[11px] font-bold transition-all rounded-xl shadow-md active:scale-95 shrink-0 cursor-pointer ${
                isAdding
                  ? "bg-slate-900 text-white hover:bg-slate-800 shadow-slate-900/10"
                  : "bg-blue-600 text-white shadow-blue-600/25 hover:bg-blue-700"
              }`}
            >
              <AppIcon name={isAdding ? "X" : "Plus"} size={16} />
              <span>{isAdding ? "Cancel" : "Add Result"}</span>
            </button>
          </div>
        </div>

        {/* Divider */}
        <div className="h-px w-full bg-slate-100" />

        {/* Tier 2: Filters Row + Records Count */}
        <div className="flex flex-wrap items-center justify-between gap-2.5">
          <div className="flex flex-wrap items-center gap-2 flex-1">
            <div className="flex items-center gap-1.5 text-slate-500 text-xs font-semibold shrink-0 mr-1">
              <AppIcon name="SlidersHorizontal" size={13} className="text-slate-500" />
              <span className="text-[10px] uppercase tracking-wider font-extrabold text-slate-500">Filters:</span>
            </div>

            {/* Class filter */}
            <select
              value={classFilter}
              onChange={(e) => {
                const val = e.target.value;
                setClassFilter(val);
                setExamFilter("all");
                setStudentFilter("all");
                setSubjectFilter("all");
                updateQuery({ class_id: val, exam_id: "all", student_id: "all", subject_id: "all" });
              }}
              className="h-8.5 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-semibold text-slate-700 outline-none cursor-pointer transition-all hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            >
              <option value="all">All Classes</option>
              {((classState.data as any)?.data || []).map((c: any) => (
                <option key={c._id} value={c._id}>
                  {c.name}
                </option>
              ))}
            </select>

            {/* Exam filter */}
            <select
              value={examFilter}
              onChange={(e) => {
                const val = e.target.value;
                setExamFilter(val);
                updateQuery({ exam_id: val });
              }}
              className="h-8.5 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-semibold text-slate-700 outline-none cursor-pointer transition-all hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10 disabled:opacity-40 disabled:cursor-not-allowed"
              disabled={classFilter === "all"}
              title={classFilter === "all" ? "Select a class first to filter by exam" : undefined}
            >
              <option value="all">All Exams / Terms</option>
              {(examListState.data || []).map((e: any) => (
                <option key={e._id} value={e._id}>
                  {e.title}
                </option>
              ))}
            </select>

            {/* Student filter */}
            <select
              value={studentFilter}
              onChange={(e) => {
                const val = e.target.value;
                setStudentFilter(val);
                updateQuery({ student_id: val });
              }}
              className="h-8.5 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-semibold text-slate-700 outline-none cursor-pointer transition-all hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            >
              <option value="all">All Students</option>
              {studentOptions
                .filter((s) => classFilter === "all" || s.class_id === classFilter)
                .map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))}
            </select>

            {/* Subject filter */}
            <select
              value={subjectFilter}
              onChange={(e) => {
                const val = e.target.value;
                setSubjectFilter(val);
                updateQuery({ subject_id: val });
              }}
              className="h-8.5 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-semibold text-slate-700 outline-none cursor-pointer transition-all hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            >
              <option value="all">All Subjects</option>
              {subjectOptions.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.label}
                </option>
              ))}
            </select>

            {/* Date filter */}
            <input
              type="date"
              value={dateFilter}
              onChange={(e) => {
                const val = e.target.value;
                setDateFilter(val);
                updateQuery({ date: val });
              }}
              className="h-8.5 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-semibold text-slate-700 outline-none cursor-pointer transition-all hover:border-slate-300 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            />

            {/* Reset Filters button */}
            {hasActiveFilters && (
              <button
                type="button"
                onClick={clearAllFilters}
                className="h-8.5 px-3 rounded-lg border border-rose-200 bg-rose-50 text-rose-700 text-xs font-bold hover:bg-rose-100 transition-colors flex items-center gap-1 shadow-2xs cursor-pointer"
              >
                <AppIcon name="X" size={13} />
                Reset Filters
              </button>
            )}
          </div>

          {/* Records Count Badge */}
          <div className="flex items-center gap-2 shrink-0 ml-auto">
            <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg bg-slate-100 text-xs font-bold text-slate-700 border border-slate-200/60">
              <span className="text-blue-600 font-extrabold">{filteredRows.length}</span>
              <span className="text-slate-500">{filteredRows.length === 1 ? "Record" : "Records"}</span>
            </span>
          </div>
        </div>
      </div>

      {/* Record Form */}
      {isAdding && (
        <div className="premium-card p-6 bg-white border-blue-100 shadow-xl shadow-blue-900/5 animate-in slide-in-from-top-4 duration-300">
          <div className="mb-6">
            <h2 className="text-lg font-bold text-slate-900">Add assessment results</h2>
            <p className="text-[11px] font-bold text-slate-400 normal-case mt-1">Single student entry</p>
          </div>
          {isDependencyLoading ? (
            <div className="space-y-4">
              <Skeleton className="h-10 w-full rounded-xl" />
              <div className="grid grid-cols-2 gap-4">
                <Skeleton className="h-10 w-full rounded-xl" />
                <Skeleton className="h-10 w-full rounded-xl" />
              </div>
            </div>
          ) : (
            <ResultForm
              examOptions={examOptions}
              studentOptions={studentOptions}
              onCreate={async (data) => {
                await addResult(data);
                setIsAdding(false);
              }}
            />
          )}
        </div>
      )}

      {/* Canva Templates */}
      {templatesState.status !== "loading" &&
        templatesState.status !== "idle" &&
        resultTemplates.length > 0 && (
          <div className="space-y-4 no-print mb-6">
            <div className="flex items-center gap-2">
              <div className="h-6 w-6 rounded-lg bg-indigo-600 text-white flex items-center justify-center">
                <AppIcon name="Palette" size={14} />
              </div>
              <h2 className="text-xs font-black text-slate-800 uppercase tracking-widest">
                Canva Designed Result Card Layouts
              </h2>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
              {resultTemplates.map((template) => (
                <div
                  key={template._id}
                  className="bg-white rounded-2xl border border-slate-200 p-4 shadow-sm hover:shadow-md transition-all flex flex-col justify-between group relative overflow-hidden"
                >
                  <div className="absolute right-0 top-0 w-24 h-24 bg-gradient-to-br from-indigo-500/10 to-purple-500/10 rounded-full blur-xl pointer-events-none" />
                  <div>
                    <div className="flex items-start justify-between mb-2">
                      <span className="px-2 py-0.5 rounded-full bg-indigo-50 text-indigo-600 border border-indigo-100 text-[9px] font-black uppercase tracking-widest">
                        Canva Template
                      </span>
                      <span className="text-[9px] font-bold text-slate-400 capitalize">{template.orientation}</span>
                    </div>
                    <h3 className="font-bold text-slate-900 text-xs mb-1 group-hover:text-indigo-600 transition-colors">
                      {template.name}
                    </h3>
                    <p className="text-[10px] text-slate-400 font-medium">
                      Designed on {new Date(template.updated_at).toLocaleDateString()}
                    </p>
                  </div>
                  <div className="mt-4 pt-3 border-t border-slate-100 flex gap-2">
                    <button
                      type="button"
                      onClick={() => navigate(`/admin/templates/edit/${template._id}`)}
                      className="flex-1 h-8 rounded-lg border border-slate-200 text-[9px] font-black uppercase tracking-widest text-slate-600 hover:bg-slate-50 transition-colors"
                    >
                      Edit Layout
                    </button>
                    <button
                      type="button"
                      onClick={() => setSelectedTemplate(template)}
                      className="flex-1 h-8 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-[9px] font-black uppercase tracking-widest text-white shadow-sm transition-colors"
                    >
                      Print Marks Cards
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

      {/* Main Content */}
      <div>
        {state.status === "loading" || state.status === "idle" ? (
          <TableSkeleton />
        ) : filteredRows.length === 0 ? (
          hasActiveFilters && (state.data || []).length > 0 ? (
            <DataState
              variant="empty"
              title="No matching results found"
              message="No assessment results match your current search or selected filters. Try clearing filters to view all records."
              action={
                <button
                  type="button"
                  onClick={clearAllFilters}
                  className="inline-flex h-9 items-center gap-2 px-4 text-xs font-bold text-slate-700 bg-white hover:bg-slate-50 rounded-xl transition-all border border-slate-200 shadow-xs cursor-pointer active:scale-95"
                >
                  <AppIcon name="X" size={14} />
                  Clear All Filters
                </button>
              }
            />
          ) : (
            <DataState
              variant="empty"
              title="No performance data found"
              message="Start recording student assessment results to see analytics."
              action={
                <button
                  type="button"
                  onClick={() => setIsAdding(true)}
                  className="inline-flex h-9 items-center gap-2 px-4 text-xs font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-xl transition-all shadow-md shadow-blue-600/25 active:scale-95 cursor-pointer"
                >
                  <AppIcon name="Plus" size={16} />
                  Add First Result
                </button>
              }
            />
          )
        ) : viewMode === "sheet" ? (
          /* ── CLASS RESULT SHEET ── */
          <ClassResultSheet
            rows={filteredRows}
            examTitle={sheetExamTitle}
            schoolName={brandedSchoolName}
            logoUrl={logoUrl}
            onExportRow={(row) => {
              exportMarksheet(row, { schoolName: brandedSchoolName, logoUrl });
              showToast("Generating marksheet…", "info");
            }}
          />
        ) : viewMode === "grid" ? (
          /* ── GRID VIEW ── */
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
            {filteredRows.map((row) => {
              const percentage = (row.obtained_marks / row.max_marks) * 100;
              return (
                <div
                  key={row._id}
                  className="premium-card group relative flex flex-col p-0 overflow-hidden transition-all duration-500 bg-white border-slate-200/60 hover:shadow-2xl hover:shadow-slate-200/80 hover:-translate-y-1"
                >
                  <div className="p-5">
                    <div className="flex items-start justify-between mb-4">
                      <div className="h-10 w-10 rounded-xl bg-slate-900 text-white flex items-center justify-center text-xs font-bold shadow-sm">
                        {(row.student_name || "S").substring(0, 1)}
                      </div>
                      <Badge
                        variant={row.grade === "A" || row.grade === "A+" ? "success" : row.grade === "F" ? "error" : "primary"}
                        className="text-[10px] font-bold px-2 py-0.5"
                      >
                        Grade {row.grade}
                      </Badge>
                    </div>
                    <div className="mb-4">
                      <h3 className="text-lg font-bold text-slate-900 tracking-tight group-hover:text-blue-600 transition-colors">
                        {row.student_name}
                      </h3>
                      <p className="text-[10px] font-bold text-slate-400 mt-1">
                        {row.admission_no} · {row.class_name}
                      </p>
                    </div>
                    {/* Subject chips */}
                    {row.subjects && row.subjects.length > 0 && (
                      <div className="flex flex-wrap gap-1 mb-4">
                        {row.subjects.map((s) => {
                          const isAbsent = s.obtained_marks === -1;
                          const pct = s.max_marks > 0 && !isAbsent ? (s.obtained_marks / s.max_marks) * 100 : 0;
                          return (
                            <span
                              key={s.subject_id}
                              className={`inline-flex items-center gap-1 h-5 px-1.5 rounded border text-[8px] font-bold ${
                                isAbsent
                                  ? "bg-rose-50 border-rose-100 text-rose-600"
                                  : pct >= 50
                                  ? "bg-emerald-50 border-emerald-100 text-emerald-700"
                                  : "bg-amber-50 border-amber-100 text-amber-700"
                              }`}
                            >
                              {s.subject_name.slice(0, 4)}
                              <span className="opacity-60">{isAbsent ? "AB" : `${s.obtained_marks}`}</span>
                            </span>
                          );
                        })}
                      </div>
                    )}
                    <div className="bg-slate-50/50 rounded-xl p-3 border border-slate-100/50">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-[10px] font-bold text-slate-400">Total Score</span>
                        <span className={`text-[11px] font-bold ${percentage >= 80 ? "text-emerald-600" : "text-blue-600"}`}>
                          {percentage.toFixed(1)}%
                        </span>
                      </div>
                      <div className="h-2 w-full bg-slate-100 rounded-full overflow-hidden">
                        <div
                          className={`h-full transition-all duration-700 rounded-full ${percentage >= 80 ? "bg-emerald-500" : "bg-blue-500"}`}
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                      <p className="text-[10px] font-bold text-slate-500 mt-2 text-center">
                        {row.obtained_marks} <span className="text-slate-300">/ {row.max_marks}</span>
                      </p>
                    </div>
                  </div>
                  <div className="mt-auto px-5 py-4 bg-slate-50/50 border-t border-slate-100 flex items-center justify-between group-hover:bg-white transition-all">
                    <button
                      onClick={() => exportMarksheet(row, { schoolName: brandedSchoolName, logoUrl })}
                      className="text-[10px] font-bold text-slate-400 hover:text-blue-600 flex items-center gap-1 transition-colors"
                    >
                      <AppIcon name="BookOpen" size={14} />
                      Report
                    </button>
                    <button
                      onClick={() => {
                        const base = isTeacher ? "/teacher" : "/admin";
                        navigate(`${base}/results/${row._id}`);
                      }}
                      className="h-8 px-4 rounded-lg bg-blue-600 text-[10px] font-bold text-white hover:bg-blue-700 transition-all flex items-center gap-2 shadow-sm active:scale-95"
                    >
                      Details
                      <AppIcon name="QueryStats" size={14} />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          /* ── LIST VIEW ── */
          <div className="premium-card overflow-hidden border-slate-200/60 shadow-sm bg-white rounded-2xl">
            <DataTable
              columns={columns}
              rows={filteredRows}
              rowKey={(row) => row._id}
              sortable
              paginated={10}
              rowActions={rowActions}
            />
          </div>
        )}
      </div>

      {/* BulkGeneratorModal for Canva templates */}
      {selectedTemplate && (
        <BulkGeneratorModal
          isOpen={selectedTemplate !== null}
          onClose={() => setSelectedTemplate(null)}
          activeType="result_card"
          template={selectedTemplate}
          customStudents={filteredRows.map((r) => ({
            _id: r._id,
            first_name: r.student_name.split(" ")[0] || r.student_name,
            last_name: r.student_name.split(" ").slice(1).join(" ") || "",
            roll_no: r.admission_no || "N/A",
            registration_no: r.admission_no || "N/A",
            class_name: r.class_name || "N/A",
            section: "A",
            father_name: `Father of ${r.student_name}`,
            marks: String(r.obtained_marks),
            grade: r.grade || "A",
            percentage: String(Math.round((r.obtained_marks / (r.max_marks || 100)) * 100)),
            fee_amount: "—",
            due_date: "—",
            course_name: r.exam_title || "General Curriculum",
            issue_date: new Date(r.graded_at || Date.now()).toLocaleDateString(),
          }))}
        />
      )}
    </div>
  );
}
