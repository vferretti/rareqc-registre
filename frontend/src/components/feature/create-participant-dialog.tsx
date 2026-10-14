import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useForm, useFieldArray } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import api from "@/lib/api";
import { isApiConflict } from "@/lib/api-error";
import { AddressInput } from "@/components/base/address-autocomplete";
import {
  participantSchema,
  formatRAMQ,
  type ParticipantFormValues,
} from "@/lib/validations/participant";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/base/ui/dialog";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
} from "@/components/base/ui/form";
import { Plus, Trash2 } from "lucide-react";
import { useTour } from "@/hooks/useTour";
import { createParticipantTour } from "@/tours/create-participant";
import { Button } from "@/components/base/ui/button";
import { Input } from "@/components/base/ui/input";
import { Label } from "@/components/base/ui/label";
import { DatePicker } from "@/components/base/ui/date-picker";
import { Checkbox } from "@/components/base/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/base/ui/select";
import type { Participant } from "@/types/participant";
import { localizedField } from "@/lib/enum-label";
import { useEnums } from "@/hooks/useEnums";
import { LANGUAGE_OPTIONS, PROVINCE_OPTIONS } from "@/lib/constants";

interface ParticipantFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: (participantId?: number) => void;
  /** When provided, the dialog opens in edit mode */
  participant?: Participant | null;
}

const DEFAULT_VALUES: ParticipantFormValues = {
  first_name: "",
  last_name: "",
  date_of_birth: "",
  sex_at_birth_code: "",
  city_of_birth: "",
  ramq: "",
  vital_status_code: "alive",
  date_of_death: "",
  email: "",
  phone: "",
  apartment_number: "",
  street_address: "",
  city: "",
  province: "QC",
  code_postal: "",
  preferred_language: "fr",
  contacts: [],
};

function participantToFormValues(p: Participant): ParticipantFormValues {
  const selfContact = p.contacts?.find((c) => c.relationship_code === "self");
  return {
    first_name: p.first_name,
    last_name: p.last_name,
    date_of_birth: p.date_of_birth ? p.date_of_birth.slice(0, 10) : "",
    sex_at_birth_code: p.sex_at_birth_code,
    city_of_birth: p.city_of_birth ?? "",
    ramq: p.ramq ?? "",
    vital_status_code: p.vital_status_code,
    date_of_death: p.date_of_death ? p.date_of_death.slice(0, 10) : "",
    email: selfContact?.email ?? "",
    phone: selfContact?.phone ?? "",
    apartment_number: selfContact?.apartment_number ?? "",
    street_address: selfContact?.street_address ?? "",
    city: selfContact?.city ?? "",
    province: selfContact?.province ?? "QC",
    code_postal: selfContact?.code_postal ?? "",
    preferred_language: selfContact?.preferred_language ?? "fr",
    contacts: [],
  };
}

export function ParticipantFormDialog({
  open,
  onOpenChange,
  onSuccess,
  participant,
}: ParticipantFormDialogProps) {
  const { t, i18n } = useTranslation();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const { enums } = useEnums();
  const isEdit = !!participant;
  const { TourButton, TourOverlay } = useTour(
    isEdit ? [] : createParticipantTour,
  );
  const lang = i18n.language;

  const schema = participantSchema(t);

  const form = useForm<ParticipantFormValues>({
    resolver: zodResolver(schema),
    defaultValues: DEFAULT_VALUES,
    mode: "onSubmit",
    reValidateMode: "onChange",
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "contacts",
  });

  /** Toggles primary: only one contact can be primary at a time. */
  const handleTogglePrimaryContact = (index: number) => {
    const contacts = form.getValues("contacts") ?? [];
    const isAlreadyPrimary = contacts[index]?.is_primary;
    contacts.forEach((_, i) => {
      form.setValue(
        `contacts.${i}.is_primary`,
        !isAlreadyPrimary && i === index,
      );
    });
  };

  const handleAddContact = () => {
    append({
      first_name: "",
      last_name: "",
      relationship_code: "",
      preferred_language: "fr",
      same_coordinates: false,
      is_primary: false,
      email: "",
      phone: "",
      apartment_number: "",
      street_address: "",
      city: "",
      province: "QC",
      code_postal: "",
    });
  };

  // Reset form when participant changes (edit mode) or when switching modes
  useEffect(() => {
    if (open) {
      if (participant) {
        const values = participantToFormValues(participant);
        form.reset(values);
      } else {
        form.reset(DEFAULT_VALUES);
      }
    }
  }, [open, participant, form]);

  const handleOpenChange = (value: boolean) => {
    if (!value) {
      form.reset(DEFAULT_VALUES);
      setSubmitError(null);
    }
    onOpenChange(value);
  };

  const onSubmit = async (data: ParticipantFormValues) => {
    setSubmitError(null);
    try {
      let createdId: number | undefined;
      if (isEdit) {
        await api.put(`/participants/${participant.id}`, data);
      } else {
        const res = await api.post("/participants", data);
        createdId = res.data?.id;
      }
      form.reset(DEFAULT_VALUES);
      onOpenChange(false);
      onSuccess?.(createdId);
    } catch (err) {
      const prefix = isEdit ? "edit_participant" : "create_participant";
      setSubmitError(
        t(
          isApiConflict(err)
            ? `${prefix}.error_duplicate_ramq`
            : `${prefix}.error`,
        ),
      );
    }
  };

  const vitalStatus = form.watch("vital_status_code");

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <div className="flex items-center gap-3 pr-6">
            <DialogTitle>
              {t(
                isEdit ? "edit_participant.title" : "create_participant.title",
              )}
            </DialogTitle>
            {!isEdit && <TourButton />}
          </div>
        </DialogHeader>

        {!isEdit && <TourOverlay />}

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            noValidate
            autoComplete="off"
            className="space-y-6"
          >
            {/* Section 1: Participant */}
            <fieldset className="space-y-4" data-tour="form-identity">
              <legend className="text-sm font-semibold text-foreground">
                {t("create_participant.section_participant")}
              </legend>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="first_name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.first_name")}
                      </FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="last_name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.last_name")}</FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="date_of_birth"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.date_of_birth")}
                      </FormLabel>
                      <FormControl>
                        <DatePicker
                          value={field.value || undefined}
                          onChange={(v) => field.onChange(v ?? "")}
                          maxDate={new Date()}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="sex_at_birth_code"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.sex_at_birth")}
                      </FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {enums?.sex_at_birth.map((e) => (
                            <SelectItem key={e.code} value={e.code}>
                              {localizedField(e, "name", lang)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="ramq"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.ramq")}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          onChange={(e) =>
                            field.onChange(formatRAMQ(e.target.value))
                          }
                          placeholder={t("create_participant.ramq_placeholder")}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="city_of_birth"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.city_of_birth")}
                      </FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="vital_status_code"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.vital_status")}
                      </FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {enums?.vital_status.map((e) => (
                            <SelectItem key={e.code} value={e.code}>
                              {localizedField(e, "name", lang)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormItem>
                  )}
                />
                {vitalStatus === "deceased" && (
                  <FormField
                    schema={schema}
                    control={form.control}
                    name="date_of_death"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t("create_participant.date_of_death")}
                        </FormLabel>
                        <FormControl>
                          <DatePicker
                            value={field.value || undefined}
                            onChange={(v) => field.onChange(v ?? "")}
                            maxDate={new Date()}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                )}
              </div>
            </fieldset>

            <hr className="border-border" />

            {/* Section 2: Coordonnées */}
            <fieldset className="space-y-4" data-tour="form-coordinates">
              <legend className="text-sm font-semibold text-foreground">
                {t("create_participant.section_coordinates")}
              </legend>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="email"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.email")}</FormLabel>
                      <FormControl>
                        <Input type="email" {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="phone"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.phone")}</FormLabel>
                      <FormControl>
                        <Input type="tel" {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="street_address"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.street_address")}
                      </FormLabel>
                      <FormControl>
                        <AddressInput
                          value={field.value}
                          onChange={field.onChange}
                          onAddressSelect={(addr) => {
                            form.setValue(
                              "apartment_number",
                              addr.apartment_number,
                            );
                            form.setValue("city", addr.city);
                            form.setValue("province", addr.province || "QC");
                            form.setValue("code_postal", addr.code_postal);
                          }}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="apartment_number"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.apartment_number")}
                      </FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="city"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.city")}</FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="province"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t("create_participant.province")}</FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {PROVINCE_OPTIONS.map((code) => (
                            <SelectItem key={code} value={code}>
                              {t(`enums.province.${code}`)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormItem>
                  )}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField
                  schema={schema}
                  control={form.control}
                  name="code_postal"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.code_postal")}
                      </FormLabel>
                      <FormControl>
                        <Input {...field} />
                      </FormControl>
                    </FormItem>
                  )}
                />
                <FormField
                  schema={schema}
                  control={form.control}
                  name="preferred_language"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("create_participant.preferred_language")}
                      </FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {LANGUAGE_OPTIONS.map((code) => (
                            <SelectItem key={code} value={code}>
                              {t(`enums.language.${code}`)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormItem>
                  )}
                />
              </div>
            </fieldset>

            {/* Section 3: Contacts — create mode only */}
            {!isEdit && (
              <>
                <hr className="border-border" />
                <fieldset className="space-y-4" data-tour="form-contacts">
                  <legend className="text-sm font-semibold text-foreground">
                    {t("create_participant.section_contacts")}
                  </legend>

                  {fields.map((field, index) => {
                    const sameCoordinates = form.watch(
                      `contacts.${index}.same_coordinates`,
                    );
                    return (
                      <div
                        key={field.id}
                        className="space-y-3 rounded-md border border-border p-4"
                      >
                        <div className="flex items-center justify-between">
                          <p className="text-sm font-semibold">
                            {t("create_participant.contact_ordinal", {
                              number: index + 1,
                            })}
                          </p>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => remove(index)}
                            className="text-destructive hover:text-destructive"
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                          <FormField
                            schema={null}
                            control={form.control}
                            name={`contacts.${index}.first_name`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>
                                  {t("create_participant.first_name")}
                                </FormLabel>
                                <FormControl>
                                  <Input {...field} />
                                </FormControl>
                              </FormItem>
                            )}
                          />
                          <FormField
                            schema={null}
                            control={form.control}
                            name={`contacts.${index}.last_name`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>
                                  {t("create_participant.last_name")}
                                </FormLabel>
                                <FormControl>
                                  <Input {...field} />
                                </FormControl>
                              </FormItem>
                            )}
                          />
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                          <FormField
                            schema={null}
                            control={form.control}
                            name={`contacts.${index}.relationship_code`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>
                                  {t("create_participant.relationship")}
                                </FormLabel>
                                <Select
                                  value={field.value}
                                  onValueChange={field.onChange}
                                >
                                  <FormControl>
                                    <SelectTrigger>
                                      <SelectValue />
                                    </SelectTrigger>
                                  </FormControl>
                                  <SelectContent>
                                    {enums?.relationship
                                      .filter((e) => e.code !== "self")
                                      .map((e) => (
                                        <SelectItem key={e.code} value={e.code}>
                                          {localizedField(e, "name", lang)}
                                        </SelectItem>
                                      ))}
                                  </SelectContent>
                                </Select>
                              </FormItem>
                            )}
                          />
                          <FormField
                            schema={null}
                            control={form.control}
                            name={`contacts.${index}.preferred_language`}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel>
                                  {t("create_participant.preferred_language")}
                                </FormLabel>
                                <Select
                                  value={field.value}
                                  onValueChange={field.onChange}
                                >
                                  <FormControl>
                                    <SelectTrigger>
                                      <SelectValue />
                                    </SelectTrigger>
                                  </FormControl>
                                  <SelectContent>
                                    {LANGUAGE_OPTIONS.map((code) => (
                                      <SelectItem key={code} value={code}>
                                        {t(`enums.language.${code}`)}
                                      </SelectItem>
                                    ))}
                                  </SelectContent>
                                </Select>
                              </FormItem>
                            )}
                          />
                        </div>

                        <div className="flex items-center gap-2">
                          <Checkbox
                            id={`c-${index}-is-primary`}
                            checked={form.watch(`contacts.${index}.is_primary`)}
                            onCheckedChange={() =>
                              handleTogglePrimaryContact(index)
                            }
                          />
                          <Label
                            htmlFor={`c-${index}-is-primary`}
                            className="font-normal"
                          >
                            {t("create_participant.is_primary")}
                          </Label>
                        </div>

                        <hr className="border-border" />

                        <p className="text-sm font-semibold text-foreground">
                          {t("create_participant.section_coordinates")}
                        </p>

                        <div className="flex items-center gap-2">
                          <Checkbox
                            id={`c-${index}-same-coordinates`}
                            checked={sameCoordinates}
                            onCheckedChange={(checked) => {
                              const isSame = checked === true;
                              form.setValue(
                                `contacts.${index}.same_coordinates`,
                                isSame,
                              );
                              if (isSame) {
                                form.setValue(
                                  `contacts.${index}.email`,
                                  form.getValues("email"),
                                );
                                form.setValue(
                                  `contacts.${index}.phone`,
                                  form.getValues("phone"),
                                );
                                form.setValue(
                                  `contacts.${index}.apartment_number`,
                                  form.getValues("apartment_number"),
                                );
                                form.setValue(
                                  `contacts.${index}.street_address`,
                                  form.getValues("street_address"),
                                );
                                form.setValue(
                                  `contacts.${index}.city`,
                                  form.getValues("city"),
                                );
                                form.setValue(
                                  `contacts.${index}.province`,
                                  form.getValues("province"),
                                );
                                form.setValue(
                                  `contacts.${index}.code_postal`,
                                  form.getValues("code_postal"),
                                );
                              }
                            }}
                          />
                          <Label
                            htmlFor={`c-${index}-same-coordinates`}
                            className="font-normal"
                          >
                            {t("create_participant.same_coordinates")}
                          </Label>
                        </div>

                        <div className="space-y-3">
                          <div className="grid grid-cols-2 gap-4">
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.email`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.email")}
                                  </FormLabel>
                                  <FormControl>
                                    <Input
                                      type="email"
                                      {...field}
                                      disabled={sameCoordinates}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.phone`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.phone")}
                                  </FormLabel>
                                  <FormControl>
                                    <Input
                                      type="tel"
                                      {...field}
                                      disabled={sameCoordinates}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                          </div>
                          <div className="grid grid-cols-2 gap-4">
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.street_address`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.street_address")}
                                  </FormLabel>
                                  <FormControl>
                                    <AddressInput
                                      value={field.value}
                                      onChange={field.onChange}
                                      disabled={sameCoordinates}
                                      onAddressSelect={(addr) => {
                                        form.setValue(
                                          `contacts.${index}.apartment_number`,
                                          addr.apartment_number,
                                        );
                                        form.setValue(
                                          `contacts.${index}.city`,
                                          addr.city,
                                        );
                                        form.setValue(
                                          `contacts.${index}.province`,
                                          addr.province || "QC",
                                        );
                                        form.setValue(
                                          `contacts.${index}.code_postal`,
                                          addr.code_postal,
                                        );
                                      }}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.apartment_number`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.apartment_number")}
                                  </FormLabel>
                                  <FormControl>
                                    <Input
                                      {...field}
                                      disabled={sameCoordinates}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                          </div>
                          <div className="grid grid-cols-2 gap-4">
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.city`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.city")}
                                  </FormLabel>
                                  <FormControl>
                                    <Input
                                      {...field}
                                      disabled={sameCoordinates}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.province`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.province")}
                                  </FormLabel>
                                  <Select
                                    value={field.value}
                                    onValueChange={field.onChange}
                                    disabled={sameCoordinates}
                                  >
                                    <FormControl>
                                      <SelectTrigger>
                                        <SelectValue />
                                      </SelectTrigger>
                                    </FormControl>
                                    <SelectContent>
                                      {PROVINCE_OPTIONS.map((code) => (
                                        <SelectItem key={code} value={code}>
                                          {t(`enums.province.${code}`)}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </FormItem>
                              )}
                            />
                          </div>
                          <div className="grid grid-cols-2 gap-4">
                            <FormField
                              schema={null}
                              control={form.control}
                              name={`contacts.${index}.code_postal`}
                              render={({ field }) => (
                                <FormItem>
                                  <FormLabel>
                                    {t("create_participant.code_postal")}
                                  </FormLabel>
                                  <FormControl>
                                    <Input
                                      {...field}
                                      disabled={sameCoordinates}
                                    />
                                  </FormControl>
                                </FormItem>
                              )}
                            />
                            <div />
                          </div>
                        </div>
                      </div>
                    );
                  })}

                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAddContact}
                  >
                    <Plus className="mr-1 size-4" />
                    {t("create_participant.add_contact")}
                  </Button>
                </fieldset>
              </>
            )}

            {submitError && (
              <p className="text-sm text-destructive">{submitError}</p>
            )}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => handleOpenChange(false)}
                disabled={form.formState.isSubmitting}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {t(
                  isEdit
                    ? "edit_participant.submit"
                    : "create_participant.submit",
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
