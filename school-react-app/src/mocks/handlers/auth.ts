import { http, HttpResponse } from "msw";
import { buildMockJwt } from "../auth-token";
import { db } from "../db";
import type { Role } from "@/types/auth";

interface LoginBody {
  email: string;
  password: string;
  role?: Role;
}

interface SignupBody {
  fullName: string;
  email: string;
  password: string;
  confirmPassword: string;
  role: Role;
  schoolName?: string;
  schoolCode?: string;
}

const ROLE_ALIASES: Record<string, Role> = {
  admin: "admin",
  teacher: "teacher",
  student: "student",
  super_admin: "super_admin",
};

export const authHandlers = [
  http.post("/api/auth/login", async ({ request }) => {
    const body = (await request.json()) as LoginBody;
    if (!body.email || !body.password) {
      return HttpResponse.json(
        { ok: false, message: "Email and password are required" },
        { status: 400 }
      );
    }

    const role = ROLE_ALIASES[(body.role ?? "admin").toLowerCase()] ?? "admin";
    const activeYear = db.academicYears.find((ay) => ay.is_active)?._id;

    const token = buildMockJwt({
      sub: `user_${role}`,
      email: body.email,
      role,
      schoolId: db.schoolId,
      activeAcademicYearId: activeYear,
      permissions: [],
    });

    return HttpResponse.json({
      ok: true,
      data: {
        role,
        token,
        user_id: `user_${role}`,
        email: body.email,
        school_id: db.schoolId,
        active_academic_year_id: activeYear,
      },
    });
  }),

  http.post("/api/auth/signup", async ({ request }) => {
    const body = (await request.json()) as SignupBody;
    if (!body.email || !body.password) {
      return HttpResponse.json(
        {
          ok: false,
          error: { code: "VALIDATION", message: "Email and password are required" },
        },
        { status: 400 }
      );
    }
    return HttpResponse.json({ ok: true, data: { email: body.email } });
  }),

  // Auth utility shims — match old-app/api/[...slug]/route.ts behaviour for
  // pre-login probes so the UI never sees a 404 during boot.
  http.get("/api/auth/session", () => HttpResponse.json(null)),
  http.post("/api/auth/_log", () => HttpResponse.json({ ok: true })),

  // Google OAuth status — UI calls this on the live-class page; return
  // disconnected so the connect button shows.
  http.get("/api/auth/google/status", () =>
    HttpResponse.json({ ok: true, data: { connected: false } })
  ),
];
