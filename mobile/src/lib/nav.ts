import { router } from 'expo-router';
import { colors } from './theme';

const ALERT_ICON: Record<string, [string, string, string]> = {
  budget_over: ['warning-outline', colors.warning, colors.warningSoft],
  budget_warning: ['wallet-outline', colors.warning, colors.warningSoft],
  purchase_check: ['help-circle-outline', colors.negative, colors.negativeSoft],
  connection_expiring: ['alert-circle-outline', colors.negative, colors.negativeSoft],
  low_balance_before_payment: ['alert-circle-outline', colors.negative, colors.negativeSoft],
  income_late: ['time-outline', colors.warning, colors.warningSoft],
  bill_changed: ['flash-outline', colors.brand, colors.accentSoft],
  new_subscription: ['tv-outline', colors.brand, colors.accentSoft],
  seasonal_reminder: ['calendar-outline', colors.brand, colors.accentSoft],
  streak_milestone: ['flame-outline', colors.text, colors.highlightSoft],
};
export const alertIcon = (type: string): [string, string, string] => ALERT_ICON[type] ?? ['notifications-outline', colors.brand, colors.accentSoft];

/** Where an alert's action leads. */
export function openAlertTarget(screen: string, targetId: string | null) {
  switch (screen) {
    case 'budget':
      return router.push('/budgets');
    case 'transaction':
      return targetId ? router.push(`/transaction/${targetId}`) : undefined;
    case 'recurring':
      return targetId ? router.push(`/recurring/${targetId}`) : router.replace('/bills');
    case 'connections':
      return router.push('/banks');
    case 'accounts':
      return targetId ? router.push(`/account/${targetId}`) : undefined;
    default:
      return undefined;
  }
}

