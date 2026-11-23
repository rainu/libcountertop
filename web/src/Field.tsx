import { useEffect, useState, useRef } from "react";
import { Button, Stack, Group, PasswordInput, Select, Slider, Switch, Text, TextInput } from "@mantine/core";
import type { Field as FieldType } from "./api";
import { DeviceFilePicker } from "./DeviceFilePicker";

type Props = { field: FieldType; onChange: (value: string) => Promise<void>; selectFile?: (value:string)=>Promise<void> };

export function FieldRow({ field, onChange, selectFile }: Props) {
  const wide = field.kind === "number" || field.kind === "text" || field.kind === "secret" ||
    field.kind === "credential" ||
    field.kind === "list" || field.kind === "pair" || field.kind === "triple";
  return (
    <Group justify="space-between" wrap="nowrap" gap="lg" py={10}>
      <Text size="sm" style={{ flex: "1 1 auto", minWidth: 0 }}>
        {field.title}
      </Text>
      <div style={{ flex: wide ? "0 1 320px" : "0 0 auto", minWidth: wide ? 160 : undefined }}>
        <Control field={field} onChange={onChange} selectFile={selectFile} />
      </div>
    </Group>
  );
}

function Control({ field, onChange, selectFile }: Props) {
  switch (field.kind) {
    case "toggle":
      return (
        <Switch
          checked={field.value === "on"}
          onChange={(e) => onChange(e.currentTarget.checked ? "on" : "off")}
        />
      );
    case "number":
      return <Level field={field} onChange={onChange} />;
    case "choice":
      return (
        <Select
          size="sm"
          data={(field.choices ?? []).map((c) => ({ value: c.value, label: c.label }))}
          value={field.value}
          onChange={(v) => v !== null && onChange(v)}
          allowDeselect={false}
          comboboxProps={{ withinPortal: true }}
        />
      );
    case "secret":
      return <Words field={field} onChange={onChange} secret />;
    case "credential":
      return <Words field={field} onChange={onChange} masked />;
    default:
      return <Words field={field} onChange={onChange} selectFile={selectFile} />;
  }
}

function Level({ field, onChange }: Props) {
  const min = field.min ?? 0;
  const max = field.max ?? 100;
  const [value, setValue] = useState(Number(field.value) || 0);
  useEffect(() => setValue(Number(field.value) || 0), [field.value]);
  return (
    <Group gap="sm" wrap="nowrap">
      <Slider
        style={{ flex: 1 }}
        min={min}
        max={max}
        value={value}
        onChange={setValue}
        onChangeEnd={(v) => onChange(String(v))}
        label={null}
      />
      <Text size="sm" ff="monospace" w={64} ta="right">
        {value}
        {field.unit ? ` ${field.unit}` : ""}
      </Text>
    </Group>
  );
}

function Words({ field, onChange, selectFile, secret, masked }: Props & { secret?: boolean; masked?: boolean }) {
  const shown = secret ? "" : field.value;
  const [value, setValue] = useState(shown);
  useEffect(() => setValue(shown), [shown]);
  const pending = useRef(Promise.resolve());
 const [picking,setPicking]=useState(false);
 const pickerActive=useRef(false);
  const send = (v: string, strict=false) => {
    const next = pending.current.catch(() => {}).then(() => (strict ? selectFile ?? onChange : onChange)(v));
    pending.current = next;
    return next;
  };
  const commit = () => {
    if(pickerActive.current)return;
    if (secret && value === "") return;
    if (value !== shown) void send(value).then(() => secret && setValue(""));
  };
  const props = {
    size: "sm" as const,
    placeholder: field.example,
    value,
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => setValue(e.currentTarget.value),
    onBlur: commit,
    onKeyDown: (e: React.KeyboardEvent) => e.key === "Enter" && commit(),
  };
  if (secret) return <PasswordInput {...props} placeholder={field.value === "set" ? "Set" : field.example} />;
  if (masked) return <PasswordInput {...props} />;
  return <Stack gap="xs">
    <TextInput {...props} />
    {field.file_picker && <>
     <Button fullWidth size="sm" variant="light" onMouseDown={e=>{pickerActive.current=true;e.preventDefault();}} styles={{label:{whiteSpace:"normal"}}} style={{height:"auto",minHeight:36}} onClick={()=>{pickerActive.current=true;setPicking(true);}}>{field.file_picker.text.choose_image}</Button>
     {picking&&<DeviceFilePicker config={field.file_picker} onClose={()=>{setPicking(false);pickerActive.current=false;}} onSelect={async name=>{await send(name,true);setValue(name);}} />}
    </>}
    {field.reset_label && <Button fullWidth size="sm" variant="light" styles={{ label: { whiteSpace: "normal" } }} style={{ height: "auto", minHeight: 36 }} onMouseDown={(e) => e.preventDefault()} onClick={() => void send("").then(() => setValue(""))}>{field.reset_label}</Button>}
  </Stack>;
}
