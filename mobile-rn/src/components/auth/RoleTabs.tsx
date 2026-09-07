/**
 * Role-tab selector matching the web Login experience exactly: Owner /
 * Admin / Teacher / Student. The backend re-validates the requested role
 * against the authenticated account, so the tab is only a convenience.
 */

import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Icon, type IconName } from '@/components/ui/Icon';
import { colors, radius, shadows, typography } from '@/theme/tokens';
import type { LoginRole } from '@/types/auth';

interface RoleOption {
  value: LoginRole;
  label: string;
  icon: IconName;
}

const ROLES: RoleOption[] = [
  { value: 'admin', label: 'Admin', icon: 'shield' },
  { value: 'teacher', label: 'Teacher', icon: 'graduation' },
  { value: 'student', label: 'Student', icon: 'graduation' },
];

interface RoleTabsProps {
  value: LoginRole;
  onChange: (role: LoginRole) => void;
}

export function RoleTabs({ value, onChange }: RoleTabsProps) {
  return (
    <View style={styles.row}>
      {ROLES.map((role) => {
        const active = value === role.value;
        return (
          <Pressable
            key={role.value}
            onPress={() => onChange(role.value)}
            style={[styles.tab, active && [styles.tabActive, shadows.card]]}
            android_ripple={{ color: colors.primaryLight, borderless: false }}
          >
            <Icon
              name={role.icon}
              size={22}
              color={active ? colors.primary : colors.gray400}
            />
            <Text style={[styles.label, active && styles.labelActive]} numberOfLines={1}>
              {role.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    padding: 6,
    backgroundColor: colors.gray50,
    borderRadius: radius.xl,
    borderWidth: 1,
    borderColor: colors.gray100,
    gap: 4,
  },
  tab: {
    flex: 1,
    paddingVertical: 14,
    paddingHorizontal: 8,
    borderRadius: radius.lg,
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    backgroundColor: 'transparent',
  },
  tabActive: {
    backgroundColor: colors.white,
    borderWidth: 1,
    borderColor: colors.primaryLight,
  },
  label: {
    ...typography.labelXs,
    color: colors.gray400,
    textTransform: 'uppercase',
  },
  labelActive: {
    color: colors.primary,
  },
});
