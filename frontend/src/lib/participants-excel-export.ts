import ExcelJS from "exceljs";
import { todayISO } from "@/lib/format";
import api from "@/lib/api";
import { enumLabel } from "@/lib/enum-label";
import type { EnumsResponse, ParticipantListItem } from "@/types/participant";

type TFunc = (key: string, opts?: Record<string, unknown>) => string;

interface ExportParams {
  total: number;
  sorting: { id: string; desc: boolean }[];
  debouncedSearch: string;
  consentFilter: {
    registry: string[];
    recontact: string[];
    external_linkage: string[];
  };
  extSystemFilter: string[];
  bulkIds: number[] | null;
}

/** Fetches all participants matching the current filters and generates an Excel download. */
export async function exportParticipantsExcel(
  params: ExportParams,
  enums: EnumsResponse | undefined,
  lang: string,
  t: TFunc,
): Promise<void> {
  const urlParams = new URLSearchParams({
    export: "true",
    page_index: "0",
    page_size: String(params.total || 200),
    sort_field: params.sorting[0]?.id ?? "last_name",
    sort_order: params.sorting[0]?.desc ? "desc" : "asc",
  });
  if (params.debouncedSearch) urlParams.set("search", params.debouncedSearch);
  if (params.consentFilter.registry.length > 0)
    urlParams.set("consent_registry", params.consentFilter.registry.join(","));
  if (params.consentFilter.recontact.length > 0)
    urlParams.set(
      "consent_recontact",
      params.consentFilter.recontact.join(","),
    );
  if (params.consentFilter.external_linkage.length > 0)
    urlParams.set(
      "consent_external_linkage",
      params.consentFilter.external_linkage.join(","),
    );
  if (params.extSystemFilter.length > 0)
    urlParams.set("external_system", params.extSystemFilter.join(","));
  if (params.bulkIds !== null)
    urlParams.set("participant_ids", params.bulkIds.join(","));

  const { data } = await api.get(`/participants?${urlParams.toString()}`);
  const rows: ParticipantListItem[] = data.data;

  const wb = new ExcelJS.Workbook();
  const ws = wb.addWorksheet(t("participants.title"));

  ws.addRow([
    t("participants.columns.id"),
    t("participants.columns.last_name"),
    t("participants.columns.first_name"),
    t("participants.columns.date_of_birth"),
    t("participants.columns.sex_at_birth"),
    t("participants.columns.vital_status"),
    t("participants.columns.ramq"),
    t("participants.columns.consent_registry"),
    t("participants.columns.consent_recontact"),
    t("participants.columns.consent_external_linkage"),
    t("participants.columns.created_at"),
  ]);

  for (const p of rows) {
    ws.addRow([
      p.id,
      p.last_name,
      p.first_name,
      p.date_of_birth ? new Date(p.date_of_birth).toLocaleDateString(lang) : "",
      enumLabel(enums?.sex_at_birth, p.sex_at_birth_code, lang),
      enumLabel(enums?.vital_status, p.vital_status_code, lang),
      p.ramq ?? "",
      p.consent_registry
        ? enumLabel(enums?.consent_status, p.consent_registry, lang)
        : "",
      p.consent_recontact
        ? enumLabel(enums?.consent_status, p.consent_recontact, lang)
        : "",
      p.consent_external_linkage
        ? enumLabel(enums?.consent_status, p.consent_external_linkage, lang)
        : "",
      p.created_at ? new Date(p.created_at).toLocaleDateString(lang) : "",
    ]);
  }

  // Bold header row
  ws.getRow(1).font = { bold: true };

  const buffer = await wb.xlsx.writeBuffer();
  const blob = new Blob([buffer], {
    type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `participants_${todayISO()}.xlsx`;
  a.click();
  URL.revokeObjectURL(url);
}
