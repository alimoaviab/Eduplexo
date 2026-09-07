/**
 * StudentLimitModal — shown when a school tries to add a student beyond their limit.
 *
 * Triggered by the backend returning error code "STUDENT_LIMIT_REACHED".
 * Shows a professional modal with upgrade and contact options.
 */

import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";


interface StudentLimitModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentCount?: number;
  limit?: number;
  planName?: string;
}

export function StudentLimitModal({ isOpen, onClose, currentCount, limit, planName }: StudentLimitModalProps) {
  const navigate = useNavigate();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" || user?.role === "super_admin";

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="bg-white rounded-2xl p-8 max-w-md w-full shadow-2xl transform transition-all"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Icon */}
        <div className="flex justify-center">
          <div className="w-16 h-16 bg-red-50 rounded-full flex items-center justify-center">
            <svg className="w-8 h-8 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
            </svg>
          </div>
        </div>

        {/* Title */}
        <h3 className="mt-5 text-xl font-bold text-gray-900 text-center">
          Student Limit Reached
        </h3>

        {/* Description */}
        <p className="mt-3 text-gray-600 text-center text-sm">
          {isAdmin
            ? "Your school has reached its subscription student limit. Please upgrade your plan to add more students."
            : "Your school has reached its subscription student limit. Please contact your school administrator to upgrade the subscription plan."}
        </p>

        {/* Usage Info */}
        {currentCount !== undefined && limit !== undefined && (
          <div className="mt-5 bg-gray-50 rounded-xl p-4">
            <div className="flex justify-between text-sm">
              <span className="text-gray-600">Current Students</span>
              <span className="font-semibold text-gray-900">{currentCount}</span>
            </div>
            <div className="flex justify-between text-sm mt-2">
              <span className="text-gray-600">Plan Limit</span>
              <span className="font-semibold text-gray-900">{limit}</span>
            </div>
            {planName && (
              <div className="flex justify-between text-sm mt-2">
                <span className="text-gray-600">Current Plan</span>
                <span className="font-semibold text-blue-600 capitalize">{planName}</span>
              </div>
            )}
            {/* Progress bar */}
            <div className="mt-3 w-full bg-gray-200 rounded-full h-2">
              <div className="h-full bg-red-500 rounded-full" style={{ width: "100%" }} />
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="mt-6 space-y-3">
          {isAdmin ? (
            <button
              onClick={() => { onClose(); navigate("/admin/subscription"); }}
              className="w-full py-3 px-4 bg-blue-600 text-white rounded-xl font-bold hover:bg-blue-700 transition"
            >
              Upgrade Plan
            </button>
          ) : (
            <div className="p-3 bg-amber-50 border border-amber-100 rounded-xl text-center">
              <p className="text-xs font-semibold text-amber-800">
                Contact your School Administrator to increase student quota.
              </p>
            </div>
          )}
          <a
            href="mailto:support@eduplexo.com"
            className="block w-full py-3 px-4 text-center border border-gray-200 text-gray-700 rounded-xl font-medium hover:bg-gray-50 transition text-xs"
          >
            Contact Eduplexo Support
          </a>
          <button
            onClick={onClose}
            className="w-full text-center text-sm text-gray-500 hover:text-gray-700 py-2"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
