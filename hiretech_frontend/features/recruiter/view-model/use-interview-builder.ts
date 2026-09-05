"use client";

import { useSyncExternalStore } from "react";
import type { CreateInterviewInput } from "@/core/ports/repositories";

const STORAGE_KEY = "hiretech.interview-builder.v1";

const defaultDraft: CreateInterviewInput = {
  title: "",
  candidateEmail: "",
  candidateDisplayName: "",
  positionTitle: "",
  seniority: "mid",
  durationMinutes: 60,
  difficulty: "intermediate",
  technologyTags: [],
  language: "tr",
  questionSource: "human",
};

let currentDraft = defaultDraft;
let initialized = false;
const listeners = new Set<() => void>();

function readDraft(): CreateInterviewInput {
  if (typeof window === "undefined") return currentDraft;
  if (initialized) return currentDraft;
  try {
    const value = window.sessionStorage.getItem(STORAGE_KEY);
    currentDraft = value ? { ...defaultDraft, ...JSON.parse(value) } : defaultDraft;
  } catch {
    currentDraft = defaultDraft;
  }
  initialized = true;
  return currentDraft;
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function persist(next: CreateInterviewInput) {
  currentDraft = next;
  initialized = true;
  window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  listeners.forEach((listener) => listener());
}

export function useInterviewBuilder() {
  const draft = useSyncExternalStore(subscribe, readDraft, () => defaultDraft);

  const update = <K extends keyof CreateInterviewInput>(field: K, value: CreateInterviewInput[K]) => {
    persist({ ...readDraft(), [field]: value });
  };

  const clear = () => {
    window.sessionStorage.removeItem(STORAGE_KEY);
    currentDraft = defaultDraft;
    initialized = true;
    listeners.forEach((listener) => listener());
  };

  return { draft, update, clear };
}
