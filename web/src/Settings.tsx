import { Fragment, type ReactNode } from "react";
import { Divider, Group, Paper, Stack, Text, TextInput, Button, ThemeIcon, Title } from "@mantine/core";
import { notifications } from "./notify";
import { FieldRow } from "./Field";
import { groupID, type Section } from "./sections";

type Props = {
  section: Section;
  focus: string;
  change: (domain: string, name: string, value: string) => Promise<void>;
  extra?: ReactNode;
};

export function Settings({ section, focus, change, extra }: Props) {
  return (
    <Stack gap="lg" maw={760}>
      <Group gap="sm">
        <ThemeIcon variant="light" size="lg" radius="md">
          <section.icon size={20} stroke={1.6} />
        </ThemeIcon>
        <Title order={3}>{section.title}</Title>
      </Group>
      {extra}
      {section.groups.map((g, i) => (
        <Stack key={g.title} gap={6} id={groupID(section.slug, i)} style={{ scrollMarginTop: 72 }}>
          <Text size="xs" fw={700} c="dimmed" tt="uppercase" lts={0.6} px={4}>
            {g.title}
          </Text>
          <Paper withBorder>
            {g.entries.map(({ domain, field }, i) => (
              <Fragment key={`${domain}.${field.name}`}>
                {i > 0 && <Divider />}
                <div id={`f-${field.id}`} className="ct-row" data-focus={focus === field.id || undefined}>
                  <FieldRow
                    field={field}
                    onChange={(v) =>
                      change(domain, field.name, v).catch((e: Error) => notifications.error(e.message))
                    }
                  />
                  {domain === "access" && field.name === "webdav" && field.value === "on" && (
                    <Stack gap="xs" pb="md" data-testid="webdav-connection">
                      <TextInput label="WebDAV URL (HTTPS)" value={window.location.origin + "/api/v1/webdav/"} readOnly />
                      <Text size="sm">Folder: /api/v1/webdav/ · Filesystem: / · Username: owner · Password: EchoLocal admin password</Text>
                      <Button size="xs" variant="light" onClick={() => navigator.clipboard.writeText(window.location.origin + "/api/v1/webdav/").catch(() => notifications.error("Copy the WebDAV URL from the field above."))}>Copy WebDAV URL</Button>
                    </Stack>
                  )}
                </div>
              </Fragment>
            ))}
          </Paper>
        </Stack>
      ))}
    </Stack>
  );
}
