export type EvaluationReportStatus = "draft" | "review_required" | "ready_for_human_decision" | "published" | "rejected";
export type HumanReviewStatus = "pending" | "in_review" | "approved" | "rejected" | "changes_requested";
export type HumanReviewUrgency = "none" | "normal" | "high" | "immediate";

export interface RubricScore {
  criterion: string;
  score: number | null;
  maximum: number;
  evidence: string;
  applicable?: boolean;
  weight?: number;
  limitations?: string[];
}

export interface ConfidenceAssessment {
  score: number;
  level: "low" | "medium" | "high";
  reasons: string[];
  requiresHumanReview: boolean;
}

export interface HumanReviewAssessment {
  required: boolean;
  urgency: HumanReviewUrgency;
  reasonCodes: string[];
  status: HumanReviewStatus;
  reviewerUserId?: string | null;
  notes: string;
  completedAt?: string | null;
}

export interface EvaluationReport {
  id: string;
  interviewId: string;
  candidateAlias: string;
  overallScore: number;
  summary: string;
  strengths: string[];
  growthAreas: string[];
  rubric: RubricScore[];
  confidence: ConfidenceAssessment;
  humanReviewStatus: HumanReviewStatus;
  generatedAt: string;
  status?: EvaluationReportStatus;
  humanReview?: HumanReviewAssessment;
  evaluatorConfigurationVersion?: string;
  evidenceReferences?: string[];
  publishedAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
}

