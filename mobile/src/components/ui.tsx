import { Ionicons, MaterialCommunityIcons } from '@expo/vector-icons';
import { router, usePathname } from 'expo-router';
import React from 'react';
import {
  ActivityIndicator,
  Pressable,
  RefreshControl,
  ScrollView,
  StyleSheet,
  Switch,
  Text,
  TextStyle,
  View,
  ViewStyle,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { colors, fonts, radius } from '../lib/theme';

// ---------- Text ----------

type TProps = { children: React.ReactNode; style?: TextStyle | TextStyle[]; numberOfLines?: number };

export const H1 = ({ children, style }: TProps) => <Text style={[s.h1, style as TextStyle]}>{children}</Text>;
export const H2 = ({ children, style }: TProps) => <Text style={[s.h2, style as TextStyle]}>{children}</Text>;
export const H4 = ({ children, style, numberOfLines }: TProps) => (
  <Text numberOfLines={numberOfLines} style={[s.h4, style as TextStyle]}>{children}</Text>
);
export const Body = ({ children, style, numberOfLines }: TProps) => (
  <Text numberOfLines={numberOfLines} style={[s.body, style as TextStyle]}>{children}</Text>
);
export const Caption = ({ children, style, numberOfLines }: TProps) => (
  <Text numberOfLines={numberOfLines} style={[s.caption, style as TextStyle]}>{children}</Text>
);
export const Overline = ({ children, style }: TProps) => <Text style={[s.overline, style as TextStyle]}>{children}</Text>;

// ---------- Icons ----------

type IconSpec = { set: 'ion' | 'mci'; name: string };

const CATEGORY_ICONS: Record<string, IconSpec> = {
  groceries: { set: 'ion', name: 'cart-outline' },
  dining: { set: 'mci', name: 'silverware-fork-knife' },
  transport: { set: 'ion', name: 'car-outline' },
  utilities: { set: 'ion', name: 'flash-outline' },
  telecom: { set: 'ion', name: 'phone-portrait-outline' },
  subscriptions: { set: 'ion', name: 'tv-outline' },
  shopping: { set: 'ion', name: 'bag-handle-outline' },
  clothes: { set: 'ion', name: 'shirt-outline' },
  home: { set: 'ion', name: 'home-outline' },
  tech: { set: 'ion', name: 'laptop-outline' },
  school: { set: 'ion', name: 'school-outline' },
  assets: { set: 'ion', name: 'cube-outline' },
  other: { set: 'ion', name: 'shapes-outline' },
  health: { set: 'ion', name: 'heart-outline' },
  education: { set: 'ion', name: 'book-outline' },
  housing: { set: 'ion', name: 'home-outline' },
  loans: { set: 'mci', name: 'bank-outline' },
  fees: { set: 'ion', name: 'receipt-outline' },
  cash: { set: 'ion', name: 'cash-outline' },
  transfers_out: { set: 'ion', name: 'arrow-up-outline' },
  other_expense: { set: 'ion', name: 'ellipsis-horizontal' },
  salary: { set: 'ion', name: 'cash-outline' },
  transfers_in: { set: 'ion', name: 'arrow-down-outline' },
  other_income: { set: 'ion', name: 'add-circle-outline' },
  internal_transfer: { set: 'ion', name: 'swap-horizontal' },
  total: { set: 'ion', name: 'wallet-outline' },
  bank: { set: 'mci', name: 'bank-outline' },
};

export function Icon({ name, set = 'ion', size = 20, color = colors.brand }: { name: string; set?: 'ion' | 'mci'; size?: number; color?: string }) {
  if (set === 'mci') return <MaterialCommunityIcons name={name as any} size={size} color={color} />;
  return <Ionicons name={name as any} size={size} color={color} />;
}

export function CategoryIcon({ keyName, size = 40, tint = colors.brand, bg = colors.accentSoft }: { keyName: string; size?: number; tint?: string; bg?: string }) {
  const spec = CATEGORY_ICONS[keyName] ?? CATEGORY_ICONS.other_expense;
  return (
    <View style={{ width: size, height: size, borderRadius: size / 2, backgroundColor: bg, alignItems: 'center', justifyContent: 'center' }}>
      <Icon set={spec.set} name={spec.name} size={size * 0.48} color={tint} />
    </View>
  );
}

export function categoryIconSpec(key: string) {
  return CATEGORY_ICONS[key] ?? CATEGORY_ICONS.other_expense;
}

// ---------- Layout ----------

export function Screen({
  children,
  tabs,
  onRefresh,
  refreshing = false,
  bg = colors.bg,
  footer,
  scroll = true,
}: {
  children: React.ReactNode;
  tabs?: boolean;
  onRefresh?: () => void;
  refreshing?: boolean;
  bg?: string;
  footer?: React.ReactNode;
  scroll?: boolean;
}) {
  const insets = useSafeAreaInsets();
  const content = (
    <View style={{ paddingTop: insets.top + 12, paddingHorizontal: 18, paddingBottom: 24, gap: 14 }}>{children}</View>
  );
  return (
    <View style={{ flex: 1, backgroundColor: bg }}>
      {scroll ? (
        <ScrollView
          contentContainerStyle={{ flexGrow: 1 }}
          keyboardShouldPersistTaps="handled"
          refreshControl={onRefresh ? <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={colors.brand} /> : undefined}
        >
          {content}
        </ScrollView>
      ) : (
        <View style={{ flex: 1 }}>{content}</View>
      )}
      {footer ? <View style={{ paddingHorizontal: 18, paddingBottom: insets.bottom + 12, paddingTop: 8 }}>{footer}</View> : null}
      {tabs ? <TabBar /> : null}
    </View>
  );
}

export function Back({ label = 'Back' }: { label?: string }) {
  return (
    <Pressable onPress={() => (router.canGoBack() ? router.back() : router.replace('/home'))} hitSlop={12} style={s.back}>
      <Ionicons name="chevron-back" size={18} color={colors.textSecondary} />
      <Text style={s.backText}>{label}</Text>
    </Pressable>
  );
}

export function Card({ children, style, onPress }: { children: React.ReactNode; style?: ViewStyle | ViewStyle[]; onPress?: () => void }) {
  if (onPress) {
    return (
      <Pressable onPress={onPress} style={({ pressed }) => [s.card, style as ViewStyle, pressed && { opacity: 0.85 }]}>
        {children}
      </Pressable>
    );
  }
  return <View style={[s.card, style as ViewStyle]}>{children}</View>;
}

export function Row({ children, style }: { children: React.ReactNode; style?: ViewStyle }) {
  return <View style={[{ flexDirection: 'row', alignItems: 'center' }, style]}>{children}</View>;
}

/** A list row: icon, title + subtitle, right value + sub-value. */
export function ListRow({
  icon,
  title,
  subtitle,
  value,
  valueColor,
  subValue,
  onPress,
  chevron,
}: {
  icon?: React.ReactNode;
  title: string;
  subtitle?: string;
  value?: string;
  valueColor?: string;
  subValue?: string;
  onPress?: () => void;
  chevron?: boolean;
}) {
  return (
    <Pressable onPress={onPress} disabled={!onPress} style={({ pressed }) => [s.listRow, pressed && onPress ? { opacity: 0.7 } : null]}>
      {icon}
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={s.rowTitle} numberOfLines={1}>{title}</Text>
        {subtitle ? <Text style={s.rowSub} numberOfLines={2}>{subtitle}</Text> : null}
      </View>
      {value !== undefined || subValue ? (
        <View style={{ alignItems: 'flex-end' }}>
          {value !== undefined ? <Text style={[s.rowValue, valueColor ? { color: valueColor } : null]}>{value}</Text> : null}
          {subValue ? <Text style={s.rowSub}>{subValue}</Text> : null}
        </View>
      ) : null}
      {chevron ? <Ionicons name="chevron-forward" size={18} color={colors.textMuted} /> : null}
    </Pressable>
  );
}

export function KV({ label, value, valueColor }: { label: string; value: string; valueColor?: string }) {
  return (
    <Row style={{ justifyContent: 'space-between', paddingVertical: 7 }}>
      <Body style={{ color: colors.textSecondary }}>{label}</Body>
      <Text style={[s.kvValue, valueColor ? { color: valueColor } : null]}>{value}</Text>
    </Row>
  );
}

export function Pill({ label, tone = 'neutral', solid }: { label: string; tone?: 'neutral' | 'positive' | 'negative' | 'warning' | 'brand'; solid?: boolean }) {
  const map = {
    neutral: [colors.accentSoft, colors.brand],
    positive: [colors.positiveSoft, colors.positive],
    negative: [colors.negativeSoft, colors.negative],
    warning: [colors.warningSoft, colors.warning],
    brand: [colors.accentSoft, colors.brand],
  } as const;
  const [bg, fg] = map[tone];
  return (
    <View style={{ backgroundColor: solid ? fg : bg, paddingHorizontal: 10, paddingVertical: 4, borderRadius: 999, alignSelf: 'flex-start' }}>
      <Text style={{ color: solid ? '#fff' : fg, fontFamily: fonts.medium, fontSize: 12 }}>{label}</Text>
    </View>
  );
}

export function Chip({ label, active, onPress, icon }: { label: string; active?: boolean; onPress?: () => void; icon?: IconSpec }) {
  return (
    <Pressable onPress={onPress} style={[s.chip, active && { backgroundColor: colors.brand, borderColor: colors.brand }]}>
      {icon ? <Icon set={icon.set} name={icon.name} size={15} color={active ? '#fff' : colors.text} /> : null}
      <Text style={[s.chipText, active && { color: '#fff' }]}>{label}</Text>
    </Pressable>
  );
}

export function Button({ label, onPress, variant = 'primary', loading, disabled, small }: { label: string; onPress?: () => void; variant?: 'primary' | 'secondary'; loading?: boolean; disabled?: boolean; small?: boolean }) {
  const primary = variant === 'primary';
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled || loading}
      style={({ pressed }) => [
        small ? s.btnSmall : s.btn,
        primary ? { backgroundColor: colors.brand } : { backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.border },
        (pressed || disabled) && { opacity: 0.7 },
      ]}
    >
      {loading ? <ActivityIndicator color={primary ? '#fff' : colors.brand} /> : <Text style={[small ? s.btnTextSmall : s.btnText, { color: primary ? '#fff' : colors.text }]}>{label}</Text>}
    </Pressable>
  );
}

export function Progress({ pct, color = colors.brand, extraPct = 0, extraColor = colors.negative }: { pct: number; color?: string; extraPct?: number; extraColor?: string }) {
  const a = Math.max(0, Math.min(100, pct));
  const b = Math.max(0, Math.min(100 - a, extraPct));
  return (
    <View style={s.track}>
      <View style={{ width: `${a}%`, backgroundColor: color, height: '100%' }} />
      {b > 0 ? <View style={{ width: `${b}%`, backgroundColor: extraColor, height: '100%' }} /> : null}
    </View>
  );
}

export function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return <Switch value={value} onValueChange={onChange} trackColor={{ true: colors.brand, false: colors.border }} thumbColor="#fff" />;
}

export function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <View style={{ gap: 10 }}>
      <H4 style={{ fontSize: 17 }}>{title}</H4>
      {children}
    </View>
  );
}

export function Loading() {
  return (
    <View style={{ paddingVertical: 60, alignItems: 'center' }}>
      <ActivityIndicator color={colors.brand} size="large" />
    </View>
  );
}

export function ErrorBox({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Card style={{ backgroundColor: colors.negativeSoft, gap: 8 }}>
      <Row style={{ gap: 8 }}>
        <Ionicons name="alert-circle-outline" size={20} color={colors.negative} />
        <Body style={{ flex: 1, color: colors.negative }}>{message}</Body>
      </Row>
      {onRetry ? <Text onPress={onRetry} style={{ color: colors.brand, fontFamily: fonts.semibold }}>Try again</Text> : null}
    </Card>
  );
}

export function Empty({ text }: { text: string }) {
  return (
    <Card style={{ alignItems: 'center', paddingVertical: 22 }}>
      <Body style={{ color: colors.textSecondary }}>{text}</Body>
    </Card>
  );
}

// ---------- Tab bar ----------

const TABS = [
  { path: '/home', label: 'Home', icon: 'home-outline', active: 'home' },
  { path: '/feed', label: 'Feed', icon: 'document-text-outline', active: 'document-text' },
  { path: '/overview', label: 'Overview', icon: 'pie-chart-outline', active: 'pie-chart' },
  { path: '/bills', label: 'Bills', icon: 'calendar-outline', active: 'calendar' },
  { path: '/profile', label: 'Profile', icon: 'person-outline', active: 'person' },
];

export function TabBar() {
  const insets = useSafeAreaInsets();
  const path = usePathname();
  return (
    <View style={[s.tabBar, { paddingBottom: insets.bottom + 6 }]}>
      {TABS.map((t) => {
        const on = path === t.path;
        return (
          <Pressable key={t.path} style={s.tab} onPress={() => !on && router.replace(t.path as any)}>
            <Ionicons name={(on ? t.active : t.icon) as any} size={22} color={on ? colors.brand : colors.textSecondary} />
            <Text style={[s.tabLabel, on && { color: colors.brand, fontFamily: fonts.semibold }]}>{t.label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const s = StyleSheet.create({
  h1: { fontFamily: fonts.heading, fontSize: 28, lineHeight: 34, letterSpacing: -0.6, color: colors.text },
  h2: { fontFamily: fonts.heading, fontSize: 22, lineHeight: 28, letterSpacing: -0.4, color: colors.text },
  h4: { fontFamily: fonts.heading, fontSize: 16, lineHeight: 22, letterSpacing: -0.2, color: colors.text },
  body: { fontFamily: fonts.body, fontSize: 15, lineHeight: 22, color: colors.text },
  caption: { fontFamily: fonts.body, fontSize: 13, lineHeight: 18, color: colors.textSecondary },
  overline: { fontFamily: fonts.semibold, fontSize: 11, lineHeight: 14, letterSpacing: 0.7, textTransform: 'uppercase', color: colors.textSecondary },
  back: { flexDirection: 'row', alignItems: 'center', gap: 2, alignSelf: 'flex-start', paddingVertical: 4 },
  backText: { fontFamily: fonts.medium, fontSize: 15, color: colors.textSecondary },
  card: { backgroundColor: colors.surface, borderRadius: radius.lg, padding: 16 },
  listRow: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 9 },
  rowTitle: { fontFamily: fonts.medium, fontSize: 15, color: colors.text },
  rowSub: { fontFamily: fonts.body, fontSize: 12.5, color: colors.textSecondary, marginTop: 2 },
  rowValue: { fontFamily: fonts.medium, fontSize: 15, color: colors.text },
  kvValue: { fontFamily: fonts.semibold, fontSize: 15, color: colors.text, flexShrink: 1, textAlign: 'right' },
  chip: { flexDirection: 'row', alignItems: 'center', gap: 6, paddingHorizontal: 13, paddingVertical: 8, borderRadius: 999, borderWidth: 1, borderColor: colors.border, backgroundColor: colors.surface },
  chipText: { fontFamily: fonts.medium, fontSize: 13.5, color: colors.text },
  btn: { height: 52, borderRadius: 999, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 20 },
  btnSmall: { height: 34, borderRadius: 999, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 14 },
  btnText: { fontFamily: fonts.semibold, fontSize: 16 },
  btnTextSmall: { fontFamily: fonts.semibold, fontSize: 13 },
  track: { height: 7, borderRadius: 999, backgroundColor: '#e9eeec', overflow: 'hidden', flexDirection: 'row' },
  tabBar: { flexDirection: 'row', backgroundColor: colors.surface, borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border, paddingTop: 8 },
  tab: { flex: 1, alignItems: 'center', gap: 3 },
  tabLabel: { fontFamily: fonts.medium, fontSize: 11.5, color: colors.textSecondary },
});
