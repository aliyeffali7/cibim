import { Ionicons } from '@expo/vector-icons';
import { Pressable, Text, View } from 'react-native';
import { Back, Caption, Card, Empty, ErrorBox, H1, Loading, Screen } from '../components/ui';
import { api, Alert } from '../lib/api';
import { timeLabel } from '../lib/format';
import { alertIcon, openAlertTarget } from '../lib/nav';
import { useApi } from '../lib/session';
import { colors, fonts } from '../lib/theme';

// 7.1 Alerts
export default function Alerts() {
  const { data, error, loading, reload, setData } = useApi<{ items: Alert[] }>('/alerts?limit=50');
  const open = async (a: Alert) => {
    if (!a.read) {
      api('POST', `/alerts/${a.id}/read`).catch(() => {});
      setData((d) => (d ? { items: d.items.map((x) => (x.id === a.id ? { ...x, read: true } : x)) } : d));
    }
    openAlertTarget(a.action.screen, a.action.targetId);
  };
  return (
    <Screen>
      <Back label="Home" />
      <H1 style={{ fontSize: 26 }}>Alerts</H1>
      {loading && !data ? <Loading /> : null}
      {error ? <ErrorBox message={error} onRetry={reload} /> : null}
      {data && !data.items.length ? <Empty text="No alerts. All quiet." /> : null}
      {data?.items.length ? (
        <Card style={{ paddingVertical: 4 }}>
          {data.items.map((a) => {
            const [icon, fg, bg] = alertIcon(a.type);
            return (
              <Pressable key={a.id} onPress={() => open(a)} style={{ flexDirection: 'row', gap: 12, paddingVertical: 12, alignItems: 'flex-start' }}>
                <View style={{ width: 38, height: 38, borderRadius: 19, backgroundColor: bg, alignItems: 'center', justifyContent: 'center' }}>
                  <Ionicons name={icon as any} size={19} color={fg} />
                </View>
                <View style={{ flex: 1, gap: 2 }}>
                  <Text style={{ fontFamily: fonts.semibold, fontSize: 15, color: colors.text }}>{a.fallbackText.title}</Text>
                  {a.fallbackText.body ? <Caption style={{ color: colors.textSecondary }}>{a.fallbackText.body}</Caption> : null}
                  <Caption style={{ color: colors.textMuted }}>{timeLabel(a.createdAt)}</Caption>
                </View>
                {!a.read ? <View style={{ width: 8, height: 8, borderRadius: 4, backgroundColor: colors.negative, marginTop: 6 }} /> : null}
              </Pressable>
            );
          })}
        </Card>
      ) : null}
    </Screen>
  );
}
