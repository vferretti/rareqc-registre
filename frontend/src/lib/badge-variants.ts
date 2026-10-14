import { CheckCircle2, XCircle, Clock, RefreshCw } from "lucide-react";

/** Badge variant mappings for enum codes, shared across list and detail views. */

export const SEX_BADGE: Record<string, "blue" | "violet" | "secondary"> = {
  male: "blue",
  female: "violet",
};

export const VITAL_STATUS_BADGE: Record<
  string,
  "green" | "destructive" | "secondary"
> = {
  alive: "green",
  deceased: "destructive",
};

export const CONSENT_STATUS_BADGE: Record<
  string,
  "green" | "secondary" | "destructive" | "amber"
> = {
  valid: "green",
  expired: "secondary",
  withdrawn: "destructive",
  replaced_by_new_version: "amber",
};

export const CONSENT_STATUS_ICON: Record<string, typeof CheckCircle2> = {
  valid: CheckCircle2,
  expired: Clock,
  withdrawn: XCircle,
  replaced_by_new_version: RefreshCw,
};

export const CONSENT_STATUS_COLOR: Record<string, string> = {
  valid: "text-green-foreground",
  expired: "text-muted-foreground",
  withdrawn: "text-destructive",
  replaced_by_new_version: "text-amber-foreground",
};

export const ACTION_BADGE: Record<
  string,
  "green" | "blue" | "amber" | "destructive" | "secondary"
> = {
  participant_created: "green",
  participant_edited: "blue",
  contact_created: "blue",
  contact_edited: "amber",
  contact_deleted: "destructive",
  consent_added: "green",
  consent_edited: "amber",
  participant_deleted: "destructive",
  data_exported: "secondary",
};
