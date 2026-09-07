import { useEffect, useState, useCallback } from "react";
import { serviceRequest } from "@/services/service-client";
import { useAuth } from "@/hooks/useAuth";

export interface Campus {
  _id?: string;
  id?: string;
  name: string;
  code?: string;
  school_id: string;
}

export function useCampusGuard() {
  const { user, loading: authLoading } = useAuth();
  const [schools, setSchools] = useState<any[]>([]);
  const [campuses, setCampuses] = useState<Campus[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [activeSchoolId, setActiveSchoolId] = useState<string>(() => {
    return window.localStorage.getItem("active_school_id") || "";
  });
  const [activeCampusId, setActiveCampusId] = useState<string>(() => {
    return window.localStorage.getItem("active_branch_id") || "";
  });

  const loadData = useCallback(async () => {
    setIsLoading(true);
    try {
      const boundSchoolId = user?.schoolId || window.localStorage.getItem("active_school_id") || "";
      const schId = boundSchoolId;
      if (schId) {
        setActiveSchoolId(schId);
        window.localStorage.setItem("active_school_id", schId);
        setSchools([{ school_id: schId, name: "Current Campus / School" }]);
      }

      // Load campuses for this school
      const url = schId ? `/api/campuses?school_id=${encodeURIComponent(schId)}` : "/api/campuses";
      const res = await serviceRequest<Campus[]>(url);
      if (res.ok && Array.isArray(res.data) && res.data.length > 0) {
        setCampuses(res.data);
        const currentBranch = window.localStorage.getItem("active_branch_id");
        if (!currentBranch || !res.data.some(c => (c._id || c.id) === currentBranch)) {
          const firstBranch = res.data[0]._id || res.data[0].id || "";
          setActiveCampusId(firstBranch);
          window.localStorage.setItem("active_branch_id", firstBranch);
        }
      } else if (schId) {
        const fallbackCampus: Campus = {
          id: `cmp_${schId}`,
          _id: `cmp_${schId}`,
          name: "Main Campus",
          school_id: schId,
          code: "MAIN",
        };
        setCampuses([fallbackCampus]);
        setActiveCampusId(fallbackCampus.id!);
        window.localStorage.setItem("active_branch_id", fallbackCampus.id!);
      } else {
        setCampuses([]);
      }
    } catch {
      setSchools([]);
      setCampuses([]);
    } finally {
      setIsLoading(false);
    }
  }, [user?.role, user?.schoolId]);

  useEffect(() => {
    if (!authLoading) {
      void loadData();
    }
  }, [authLoading, loadData]);

  const selectBranch = (branchId: string) => {
    window.localStorage.setItem("active_branch_id", branchId);
    setActiveCampusId(branchId);
  };

  const hasSchools = Boolean(user?.schoolId || activeSchoolId || schools.length > 0);

  return {
    isLoading: authLoading || isLoading,
    schools,
    hasSchools,
    campuses,
    hasCampuses: campuses.length > 0,
    activeSchoolId,
    activeCampusId,
    selectBranch,
    reloadCampuses: loadData,
  };
}
