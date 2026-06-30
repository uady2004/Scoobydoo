import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:tiktok_clone/features/auth/presentation/providers/auth_provider.dart';
import 'package:tiktok_clone/shared/providers/theme_provider.dart';

class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  String _language = 'English';
  bool _dataSaver = false;

  String _appearanceLabel(ThemeMode mode) => switch (mode) {
        ThemeMode.light => 'Light',
        ThemeMode.system => 'System',
        ThemeMode.dark => 'Dark',
      };

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0A0A0A),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0A0A0A),
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new,
              color: Colors.white, size: 18),
          onPressed: () => context.pop(),
        ),
        title: const Text(
          'Settings',
          style: TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w700,
              fontSize: 18),
        ),
        centerTitle: true,
      ),
      body: ListView(
        children: [
          // ── Account ──────────────────────────────────────────────────────
          _SectionHeader('Account'),
          _Tile(
            icon: Icons.person_outline,
            title: 'Account',
            subtitle: 'Manage account info, username, email',
            onTap: () => context.push('/settings/security'),
          ),
          _Tile(
            icon: Icons.lock_outline,
            title: 'Privacy',
            subtitle: 'Account privacy, interactions, content',
            onTap: () => context.push('/settings/privacy'),
          ),
          _Tile(
            icon: Icons.security_outlined,
            title: 'Security',
            subtitle: 'Password, two-factor authentication',
            onTap: () => context.push('/settings/security'),
          ),
          _Tile(
            icon: Icons.notifications_outlined,
            title: 'Notifications',
            subtitle: 'Push, email and in-app alerts',
            onTap: () => context.push('/settings/notifications'),
          ),

          // ── Content ───────────────────────────────────────────────────────
          _SectionHeader('Content & Display'),
          _Tile(
            icon: Icons.language_outlined,
            title: 'Language',
            subtitle: _language,
            onTap: () => _showLanguageSheet(context),
          ),
          _Tile(
            icon: Icons.dark_mode_outlined,
            title: 'Appearance',
            subtitle: _appearanceLabel(ref.watch(themeProvider)),
            onTap: () => _showAppearanceSheet(context),
          ),
          _Tile(
            icon: Icons.accessibility_outlined,
            title: 'Accessibility',
            subtitle: 'Font size, captions, reduced motion',
            onTap: () => _showAccessibilitySheet(context),
          ),
          _SwitchTile(
            icon: Icons.data_saver_off_outlined,
            title: 'Data saver',
            subtitle: 'Reduce video quality to save data',
            value: _dataSaver,
            onChanged: (v) {
              setState(() => _dataSaver = v);
              _showSnack(context, 'Data saver ${v ? 'enabled' : 'disabled'}');
            },
          ),

          // ── Interactions ──────────────────────────────────────────────────
          _SectionHeader('Interactions'),
          _Tile(
            icon: Icons.comment_outlined,
            title: 'Comments',
            subtitle: 'Filter, keyword blocking',
            onTap: () => _showCommentsSheet(context),
          ),
          _Tile(
            icon: Icons.block_outlined,
            title: 'Blocked accounts',
            subtitle: 'Manage blocked users',
            onTap: () => _showBlockedSheet(context),
          ),
          _Tile(
            icon: Icons.people_outline,
            title: 'Suggested accounts',
            subtitle: 'Control account recommendations',
            onTap: () => _showSnack(context, 'Coming soon'),
          ),

          // ── Creator ───────────────────────────────────────────────────────
          _SectionHeader('Creator'),
          _Tile(
            icon: Icons.bar_chart_outlined,
            title: 'Creator tools',
            subtitle: 'Analytics, monetisation, live',
            onTap: () => context.push('/dashboard'),
          ),
          _Tile(
            icon: Icons.monetization_on_outlined,
            title: 'TikTok Shop',
            subtitle: 'Manage your shop',
            onTap: () => context.push('/shop'),
          ),

          // ── Support ───────────────────────────────────────────────────────
          _SectionHeader('Support'),
          _Tile(
            icon: Icons.help_outline,
            title: 'Help & Support',
            onTap: () => _showHelpSheet(context),
          ),
          _Tile(
            icon: Icons.report_outlined,
            title: 'Report a problem',
            onTap: () => _showReportProblemSheet(context),
          ),
          _Tile(
            icon: Icons.info_outline,
            title: 'About',
            subtitle: 'Version 1.0.0',
            onTap: () => _showAbout(context),
          ),
          _Tile(
            icon: Icons.policy_outlined,
            title: 'Privacy Policy',
            onTap: () => _showSnack(context, 'Opening Privacy Policy...'),
          ),
          _Tile(
            icon: Icons.description_outlined,
            title: 'Terms of Service',
            onTap: () => _showSnack(context, 'Opening Terms of Service...'),
          ),
          _Tile(
            icon: Icons.cookie_outlined,
            title: 'Cookie Policy',
            onTap: () => _showSnack(context, 'Opening Cookie Policy...'),
          ),

          const SizedBox(height: 24),

          // ── Logout ────────────────────────────────────────────────────────
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: OutlinedButton(
              onPressed: () => _confirmLogout(context),
              style: OutlinedButton.styleFrom(
                foregroundColor: const Color(0xFFEE1D52),
                side: const BorderSide(color: Color(0xFFEE1D52)),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8)),
              ),
              child: const Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.logout, size: 20),
                  SizedBox(width: 8),
                  Text('Log out',
                      style: TextStyle(
                          fontSize: 15, fontWeight: FontWeight.w600)),
                ],
              ),
            ),
          ),

          const SizedBox(height: 16),

          // ── Switch account ────────────────────────────────────────────────
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: OutlinedButton(
              onPressed: () =>
                  _showSnack(context, 'Switch account — coming soon'),
              style: OutlinedButton.styleFrom(
                foregroundColor: Colors.white54,
                side: const BorderSide(color: Color(0xFF2A2A2A)),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8)),
              ),
              child: const Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.switch_account_outlined, size: 20),
                  SizedBox(width: 8),
                  Text('Switch account',
                      style: TextStyle(
                          fontSize: 15, fontWeight: FontWeight.w600)),
                ],
              ),
            ),
          ),

          const SizedBox(height: 40),
        ],
      ),
    );
  }

  void _showSnack(BuildContext context, String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: const Color(0xFF1A1A1A),
      behavior: SnackBarBehavior.floating,
    ));
  }

  void _showLanguageSheet(BuildContext context) {
    final languages = [
      'English', 'Spanish', 'French', 'German', 'Portuguese',
      'Arabic', 'Hindi', 'Japanese', 'Korean', 'Chinese',
    ];
    _showSheet(
      context,
      title: 'Language',
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: languages
            .map((l) => ListTile(
                  title: Text(l,
                      style: const TextStyle(color: Colors.white)),
                  trailing: l == _language
                      ? const Icon(Icons.check, color: Color(0xFFEE1D52))
                      : null,
                  onTap: () {
                    setState(() => _language = l);
                    Navigator.pop(context);
                    _showSnack(context, 'Language set to $l');
                  },
                ))
            .toList(),
      ),
    );
  }

  void _showAppearanceSheet(BuildContext context) {
    final current = ref.read(themeProvider);
    final notifier = ref.read(themeProvider.notifier);

    void pick(ThemeMode mode, String label) {
      notifier.setMode(mode);
      Navigator.pop(context);
      _showSnack(context, '$label mode enabled');
    }

    _showSheet(
      context,
      title: 'Appearance',
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.dark_mode, color: Colors.white),
            title: const Text('Dark', style: TextStyle(color: Colors.white)),
            trailing: current == ThemeMode.dark
                ? const Icon(Icons.check, color: Color(0xFFEE1D52))
                : null,
            onTap: () => pick(ThemeMode.dark, 'Dark'),
          ),
          ListTile(
            leading: const Icon(Icons.light_mode, color: Colors.white),
            title: const Text('Light', style: TextStyle(color: Colors.white)),
            trailing: current == ThemeMode.light
                ? const Icon(Icons.check, color: Color(0xFFEE1D52))
                : null,
            onTap: () => pick(ThemeMode.light, 'Light'),
          ),
          ListTile(
            leading: const Icon(Icons.phone_android, color: Colors.white),
            title: const Text('Use device settings',
                style: TextStyle(color: Colors.white)),
            trailing: current == ThemeMode.system
                ? const Icon(Icons.check, color: Color(0xFFEE1D52))
                : null,
            onTap: () => pick(ThemeMode.system, 'System'),
          ),
        ],
      ),
    );
  }

  void _showAccessibilitySheet(BuildContext context) {
    bool captions = false;
    bool reduceMotion = false;
    bool highContrast = false;
    String fontSize = 'Default';

    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (sheetCtx) => StatefulBuilder(
        builder: (ctx, setSheetState) => SafeArea(
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const SizedBox(height: 12),
                Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Colors.white24,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
                const SizedBox(height: 12),
                const Text('Accessibility',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w700)),
                const SizedBox(height: 8),
                ListTile(
                  title: const Text('Font size',
                      style: TextStyle(color: Colors.white)),
                  trailing: DropdownButton<String>(
                    value: fontSize,
                    dropdownColor: const Color(0xFF2A2A2A),
                    style: const TextStyle(color: Colors.white),
                    underline: const SizedBox(),
                    items: ['Small', 'Default', 'Large', 'Extra large']
                        .map((s) => DropdownMenuItem(
                            value: s, child: Text(s)))
                        .toList(),
                    onChanged: (v) {
                      if (v != null) setSheetState(() => fontSize = v);
                    },
                  ),
                ),
                SwitchListTile(
                  title: const Text('Captions',
                      style: TextStyle(color: Colors.white)),
                  subtitle: const Text('Show captions on videos',
                      style:
                          TextStyle(color: Colors.white38, fontSize: 12)),
                  value: captions,
                  onChanged: (v) => setSheetState(() => captions = v),
                  activeTrackColor: const Color(0xFFEE1D52),
                ),
                SwitchListTile(
                  title: const Text('Reduce motion',
                      style: TextStyle(color: Colors.white)),
                  subtitle: const Text(
                      'Reduce animations and transitions',
                      style:
                          TextStyle(color: Colors.white38, fontSize: 12)),
                  value: reduceMotion,
                  onChanged: (v) =>
                      setSheetState(() => reduceMotion = v),
                  activeTrackColor: const Color(0xFFEE1D52),
                ),
                SwitchListTile(
                  title: const Text('High contrast text',
                      style: TextStyle(color: Colors.white)),
                  value: highContrast,
                  onChanged: (v) =>
                      setSheetState(() => highContrast = v),
                  activeTrackColor: const Color(0xFFEE1D52),
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showCommentsSheet(BuildContext context) {
    bool filterSpam = true;
    bool filterOffensive = true;

    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (sheetCtx) => StatefulBuilder(
        builder: (ctx, setSheetState) => SafeArea(
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const SizedBox(height: 12),
                Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Colors.white24,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
                const SizedBox(height: 12),
                const Text('Comments',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w700)),
                const SizedBox(height: 8),
                SwitchListTile(
                  title: const Text('Filter spam comments',
                      style: TextStyle(color: Colors.white)),
                  value: filterSpam,
                  onChanged: (v) =>
                      setSheetState(() => filterSpam = v),
                  activeTrackColor: const Color(0xFFEE1D52),
                ),
                SwitchListTile(
                  title: const Text('Filter offensive comments',
                      style: TextStyle(color: Colors.white)),
                  value: filterOffensive,
                  onChanged: (v) =>
                      setSheetState(() => filterOffensive = v),
                  activeTrackColor: const Color(0xFFEE1D52),
                ),
                ListTile(
                  title: const Text('Keyword filters',
                      style: TextStyle(color: Colors.white)),
                  subtitle: const Text('Add words to filter',
                      style: TextStyle(
                          color: Colors.white38, fontSize: 12)),
                  trailing: const Icon(Icons.chevron_right,
                      color: Colors.white24),
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    _showSnack(context, 'Keyword filters — coming soon');
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showBlockedSheet(BuildContext context) {
    _showSheet(
      context,
      title: 'Blocked accounts',
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text(
              'You haven\'t blocked anyone yet.',
              style: TextStyle(color: Colors.white54, fontSize: 14),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () => Navigator.pop(context),
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFEE1D52),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
                child: const Text('Done',
                    style: TextStyle(color: Colors.white)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showHelpSheet(BuildContext context) {
    _showSheet(
      context,
      title: 'Help & Support',
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.book_outlined, color: Colors.white70),
            title: const Text('Help Centre',
                style: TextStyle(color: Colors.white)),
            onTap: () {
              Navigator.pop(context);
              _showSnack(context, 'Opening Help Centre...');
            },
          ),
          ListTile(
            leading: const Icon(Icons.chat_outlined, color: Colors.white70),
            title: const Text('Contact us',
                style: TextStyle(color: Colors.white)),
            onTap: () {
              Navigator.pop(context);
              _showSnack(context, 'Opening contact form...');
            },
          ),
          ListTile(
            leading:
                const Icon(Icons.feedback_outlined, color: Colors.white70),
            title: const Text('Send feedback',
                style: TextStyle(color: Colors.white)),
            onTap: () {
              Navigator.pop(context);
              _showReportProblemSheet(context);
            },
          ),
          ListTile(
            leading: const Icon(Icons.safety_check_outlined,
                color: Colors.white70),
            title: const Text('Safety Centre',
                style: TextStyle(color: Colors.white)),
            onTap: () {
              Navigator.pop(context);
              _showSnack(context, 'Opening Safety Centre...');
            },
          ),
        ],
      ),
    );
  }

  void _showReportProblemSheet(BuildContext context) {
    final controller = TextEditingController();
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(
            20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('Report a Problem',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.bold)),
            const SizedBox(height: 16),
            TextField(
              controller: controller,
              maxLines: 5,
              style: const TextStyle(color: Colors.white),
              decoration: InputDecoration(
                hintText: 'Describe the problem...',
                hintStyle: const TextStyle(color: Colors.white38),
                filled: true,
                fillColor: Colors.white10,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(8),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () {
                  Navigator.pop(ctx);
                  _showSnack(context, 'Report submitted. Thank you!');
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFEE1D52),
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8)),
                ),
                child: const Text('Submit',
                    style: TextStyle(color: Colors.white)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showAbout(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('About',
            style: TextStyle(color: Colors.white)),
        content: const Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('TikTok Clone',
                style: TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                    fontSize: 18)),
            SizedBox(height: 4),
            Text('Version 1.0.0',
                style: TextStyle(color: Colors.white54)),
            SizedBox(height: 12),
            Text('Build: 2026.06.25',
                style: TextStyle(color: Colors.white54, fontSize: 12)),
            SizedBox(height: 12),
            Text(
              'Built with Flutter & Go.\nA feature-complete short-video platform.',
              style: TextStyle(
                  color: Colors.white70, fontSize: 13, height: 1.5),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Close',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }

  void _confirmLogout(BuildContext context) {
    showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Log out',
            style: TextStyle(color: Colors.white)),
        content: const Text(
          'Are you sure you want to log out of your account?',
          style: TextStyle(color: Colors.white70),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel',
                style: TextStyle(color: Colors.white54)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ref.read(authProvider.notifier).logout();
              context.go('/login');
            },
            child: const Text('Log out',
                style: TextStyle(color: Color(0xFFEE1D52))),
          ),
        ],
      ),
    );
  }

  void _showSheet(BuildContext context,
      {required String title, required Widget child}) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 12),
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: Colors.white24,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 12),
              Text(title,
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w700)),
              const SizedBox(height: 8),
              child,
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }
}

// ── Shared widgets ────────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  final String title;
  const _SectionHeader(this.title);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 20, 16, 6),
      child: Text(
        title.toUpperCase(),
        style: const TextStyle(
          color: Colors.white38,
          fontSize: 11,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.8,
        ),
      ),
    );
  }
}

class _Tile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback onTap;

  const _Tile({
    required this.icon,
    required this.title,
    required this.onTap,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: Icon(icon, color: Colors.white70, size: 22),
      title: Text(title,
          style: const TextStyle(color: Colors.white, fontSize: 15)),
      subtitle: subtitle != null
          ? Text(subtitle!,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 12))
          : null,
      trailing: const Icon(Icons.chevron_right,
          color: Colors.white24, size: 20),
      onTap: onTap,
    );
  }
}

class _SwitchTile extends StatefulWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _SwitchTile({
    required this.icon,
    required this.title,
    required this.value,
    required this.onChanged,
    this.subtitle,
  });

  @override
  State<_SwitchTile> createState() => _SwitchTileState();
}

class _SwitchTileState extends State<_SwitchTile> {
  late bool _val;

  @override
  void initState() {
    super.initState();
    _val = widget.value;
  }

  @override
  void didUpdateWidget(_SwitchTile old) {
    super.didUpdateWidget(old);
    if (old.value != widget.value) _val = widget.value;
  }

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      secondary: Icon(widget.icon, color: Colors.white70, size: 22),
      title: Text(widget.title,
          style: const TextStyle(color: Colors.white, fontSize: 15)),
      subtitle: widget.subtitle != null
          ? Text(widget.subtitle!,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 12))
          : null,
      value: _val,
      onChanged: (v) {
        setState(() => _val = v);
        widget.onChanged(v);
      },
      activeTrackColor: const Color(0xFFEE1D52),
    );
  }
}
