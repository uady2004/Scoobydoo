import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../shared/providers/notification_provider.dart';

/// NotificationPreferencesScreen lets the user toggle every channel and
/// notification-type switch, configure quiet hours, and set the digest frequency.
class NotificationPreferencesScreen extends StatefulWidget {
  const NotificationPreferencesScreen({super.key});

  @override
  State<NotificationPreferencesScreen> createState() =>
      _NotificationPreferencesScreenState();
}

class _NotificationPreferencesScreenState
    extends State<NotificationPreferencesScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      context.read<NotificationProvider>().loadPreferences();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios, color: Colors.white, size: 18),
          onPressed: () => Navigator.of(context).pop(),
        ),
        title: const Text(
          'Notification Settings',
          style: TextStyle(
              color: Colors.white, fontSize: 17, fontWeight: FontWeight.w600),
        ),
      ),
      body: Consumer<NotificationProvider>(
        builder: (context, provider, _) {
          if (provider.prefsLoading) {
            return const Center(
              child: CircularProgressIndicator(color: Color(0xFFFE2C55)),
            );
          }

          final prefs = provider.preferences;
          if (prefs == null) {
            return const Center(
              child: Text('Unable to load preferences.',
                  style: TextStyle(color: Colors.grey)),
            );
          }

          return ListView(
            children: [
              _SectionHeader(title: 'Delivery channels'),
              _PrefToggle(
                icon: Icons.notifications_active,
                iconColor: const Color(0xFFFE2C55),
                label: 'Push notifications',
                value: prefs.pushEnabled,
                onChanged: (v) => _update({'push_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.email_outlined,
                iconColor: const Color(0xFF25F4EE),
                label: 'Email notifications',
                value: prefs.emailEnabled,
                onChanged: (v) => _update({'email_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.sms_outlined,
                iconColor: const Color(0xFF6C63FF),
                label: 'SMS notifications',
                value: prefs.smsEnabled,
                onChanged: (v) => _update({'sms_enabled': v}),
              ),
              _SectionHeader(title: 'Activity'),
              _PrefToggle(
                icon: Icons.favorite,
                iconColor: const Color(0xFFFE2C55),
                label: 'Likes',
                value: prefs.likesEnabled,
                onChanged: (v) => _update({'likes_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.comment,
                iconColor: const Color(0xFF25F4EE),
                label: 'Comments',
                value: prefs.commentsEnabled,
                onChanged: (v) => _update({'comments_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.person_add,
                iconColor: const Color(0xFF6C63FF),
                label: 'New followers',
                value: prefs.followsEnabled,
                onChanged: (v) => _update({'follows_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.alternate_email,
                iconColor: const Color(0xFFFF9500),
                label: 'Mentions',
                value: prefs.mentionsEnabled,
                onChanged: (v) => _update({'mentions_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.card_giftcard,
                iconColor: const Color(0xFFFFD700),
                label: 'Gifts',
                value: prefs.giftsEnabled,
                onChanged: (v) => _update({'gifts_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.live_tv,
                iconColor: const Color(0xFFFE2C55),
                label: 'Live streams from people I follow',
                value: prefs.livestreamEnabled,
                onChanged: (v) => _update({'livestream_enabled': v}),
              ),
              _PrefToggle(
                icon: Icons.shopping_bag_outlined,
                iconColor: const Color(0xFF34C759),
                label: 'Orders & shipping',
                value: prefs.ordersEnabled,
                onChanged: (v) => _update({'orders_enabled': v}),
              ),
              _SectionHeader(title: 'Quiet hours'),
              _PrefToggle(
                icon: Icons.bedtime_outlined,
                iconColor: Colors.indigo,
                label: 'Enable quiet hours',
                subtitle: prefs.quietHoursEnabled && prefs.quietStart != null
                    ? '${prefs.quietStart} – ${prefs.quietEnd}'
                    : null,
                value: prefs.quietHoursEnabled,
                onChanged: (v) => _update({'quiet_hours_enabled': v}),
              ),
              if (prefs.quietHoursEnabled) ...[
                _TimePickerRow(
                  label: 'Start time',
                  time: prefs.quietStart ?? '22:00',
                  onChanged: (v) => _update({'quiet_start': v}),
                ),
                _TimePickerRow(
                  label: 'End time',
                  time: prefs.quietEnd ?? '07:00',
                  onChanged: (v) => _update({'quiet_end': v}),
                ),
              ],
              _SectionHeader(title: 'Email digest'),
              _PrefToggle(
                icon: Icons.summarize_outlined,
                iconColor: Colors.teal,
                label: 'Weekly activity digest',
                subtitle: 'A summary of your account activity',
                value: prefs.digestEnabled,
                onChanged: (v) => _update({'digest_enabled': v}),
              ),
              if (prefs.digestEnabled)
                _DigestFrequencySelector(
                  current: prefs.digestFrequency ?? 'weekly',
                  onChanged: (v) => _update({'digest_frequency': v}),
                ),
              const SizedBox(height: 40),
            ],
          );
        },
      ),
    );
  }

  void _update(Map<String, dynamic> changes) {
    context.read<NotificationProvider>().updatePreferences(changes);
  }
}

// ---------------------------------------------------------------------------
// Section header
// ---------------------------------------------------------------------------

class _SectionHeader extends StatelessWidget {
  final String title;

  const _SectionHeader({required this.title});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 24, 16, 6),
      child: Text(
        title.toUpperCase(),
        style: TextStyle(
          color: Colors.grey[500],
          fontSize: 11,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.8,
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Generic preference toggle row
// ---------------------------------------------------------------------------

class _PrefToggle extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String label;
  final String? subtitle;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _PrefToggle({
    required this.icon,
    required this.iconColor,
    required this.label,
    this.subtitle,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 2),
      leading: Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(
          color: iconColor.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Icon(icon, color: iconColor, size: 18),
      ),
      title: Text(
        label,
        style: const TextStyle(
            color: Colors.white, fontSize: 14, fontWeight: FontWeight.w500),
      ),
      subtitle: subtitle != null
          ? Text(subtitle!,
              style: TextStyle(color: Colors.grey[600], fontSize: 12))
          : null,
      trailing: Switch(
        value: value,
        onChanged: onChanged,
        activeThumbColor: const Color(0xFFFE2C55),
        activeTrackColor: const Color(0xFFFE2C55).withValues(alpha: 0.35),
        inactiveThumbColor: Colors.grey[600],
        inactiveTrackColor: Colors.grey[800],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Time picker row for quiet hours
// ---------------------------------------------------------------------------

class _TimePickerRow extends StatelessWidget {
  final String label;
  final String time; // "HH:MM"
  final ValueChanged<String> onChanged;

  const _TimePickerRow({
    required this.label,
    required this.time,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final parts = time.split(':');
    final hour = int.tryParse(parts.isNotEmpty ? parts[0] : '0') ?? 0;
    final minute = int.tryParse(parts.length > 1 ? parts[1] : '0') ?? 0;
    final tod = TimeOfDay(hour: hour, minute: minute);

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 2),
      leading: const SizedBox(width: 36),
      title: Text(
        label,
        style: TextStyle(color: Colors.grey[300], fontSize: 14),
      ),
      trailing: GestureDetector(
        onTap: () async {
          final picked = await showTimePicker(
            context: context,
            initialTime: tod,
            builder: (ctx, child) => Theme(
              data: ThemeData.dark().copyWith(
                colorScheme: const ColorScheme.dark(
                  primary: Color(0xFFFE2C55),
                ),
              ),
              child: child!,
            ),
          );
          if (picked != null) {
            final formatted =
                '${picked.hour.toString().padLeft(2, '0')}:${picked.minute.toString().padLeft(2, '0')}';
            onChanged(formatted);
          }
        },
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          decoration: BoxDecoration(
            color: Colors.grey[850],
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            time,
            style: const TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w600),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Digest frequency selector
// ---------------------------------------------------------------------------

class _DigestFrequencySelector extends StatelessWidget {
  final String current;
  final ValueChanged<String> onChanged;

  const _DigestFrequencySelector({
    required this.current,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: [
          const SizedBox(width: 52), // align with toggle labels
          _FreqChip(
            label: 'Daily',
            value: 'daily',
            selected: current == 'daily',
            onTap: () => onChanged('daily'),
          ),
          const SizedBox(width: 8),
          _FreqChip(
            label: 'Weekly',
            value: 'weekly',
            selected: current == 'weekly',
            onTap: () => onChanged('weekly'),
          ),
        ],
      ),
    );
  }
}

class _FreqChip extends StatelessWidget {
  final String label;
  final String value;
  final bool selected;
  final VoidCallback onTap;

  const _FreqChip({
    required this.label,
    required this.value,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        decoration: BoxDecoration(
          color: selected
              ? const Color(0xFFFE2C55)
              : Colors.grey[850],
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? const Color(0xFFFE2C55) : Colors.grey[700]!,
          ),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: selected ? Colors.white : Colors.grey[400],
            fontSize: 13,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}
