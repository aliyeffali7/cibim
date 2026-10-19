import { router } from 'expo-router';
import { Transaction } from '../lib/api';
import { money, shortDate } from '../lib/format';
import { colors } from '../lib/theme';
import { CategoryIcon, ListRow } from './ui';

/** One transaction in a list: merchant, "Category · Bank", amount and status. */
export function TxRow({ t, showDate = true }: { t: Transaction; showDate?: boolean }) {
  const income = t.amount.amountMinor > 0;
  const sub = t.status === 'pending' ? 'Pending' : t.isInternalTransfer ? 'Not counted' : showDate ? shortDate(t.date) : undefined;
  const title = t.isInternalTransfer ? (income ? `From ${t.merchant.name}` : `To ${t.merchant.name}`) : t.merchant.name;
  return (
    <ListRow
      onPress={() => router.push(`/transaction/${t.id}`)}
      icon={<CategoryIcon keyName={t.category.key} size={38} />}
      title={t.isInternalTransfer ? (income ? 'From your other card' : 'To your other card') : title}
      subtitle={`${t.isInternalTransfer ? 'Between your accounts' : t.category.name} · ${t.bank.name}`}
      value={t.isInternalTransfer ? money({ ...t.amount, amountMinor: Math.abs(t.amount.amountMinor) }) : money(t.amount, { sign: true })}
      valueColor={t.isInternalTransfer ? colors.textSecondary : income ? colors.positive : undefined}
      subValue={sub}
    />
  );
}
