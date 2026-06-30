import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class PrivacySettingsScreen extends StatefulWidget {
  const PrivacySettingsScreen({super.key});

  @override
  State<PrivacySettingsScreen> createState() =>
      _PrivacySettingsScreenState();
}

class _PrivacySettingsScreenState extends State<PrivacySettingsScreen> {
  bool _privateAccount = false;
  bool _showLikedVideos = false;
  bool _showFollowers = true;
  bool _showFollowing = true;
  bool _allowDuets = true;
  bool _allowStitch = true;
  bool _allowDownloads = true;
  bool _allowSuggest = true;
  bool _allowAds = false;
  bool _allowFindByContact = true;
  String _whoCanSendMessages = 'Friends';
  String _whoCanComment = 'Everyone';
  String _whoCanViewProfile = 'Everyone';
  String _whoCanDuet = 'Everyone';

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
        title: const Text('Privacy',
            style: TextStyle(
                color: Colors.white, fontWeight: FontWeight.w700)),
        centerTitle: true,
      ),
      body: ListView(
        children: [
          // ── Account Privacy ──────────────────────────────────────────────
          _Header('Account Privacy'),
          _Switch(
            title: 'Private account',
            subtitle:
                'Only approved followers can see your videos and likes',
            value: _privateAccount,
            onChanged: (v) => setState(() => _privateAccount = v),
          ),
          _Select(
            title: 'Who can view your profile',
            value: _whoCanViewProfile,
            options: ['Everyone', 'Friends', 'Only me'],
            onChanged: (v) =>
                setState(() => _whoCanViewProfile = v),
          ),
          _Switch(
            title: 'Show liked videos',
            subtitle: 'Let others see videos you\'ve liked',
            value: _showLikedVideos,
            onChanged: (v) => setState(() => _showLikedVideos = v),
          ),
          _Switch(
            title: 'Show followers list',
            value: _showFollowers,
            onChanged: (v) => setState(() => _showFollowers = v),
          ),
          _Switch(
            title: 'Show following list',
            value: _showFollowing,
            onChanged: (v) => setState(() => _showFollowing = v),
          ),

          // ── Interactions ─────────────────────────────────────────────────
          _Header('Interactions'),
          _Select(
            title: 'Who can send messages',
            value: _whoCanSendMessages,
            options: ['Everyone', 'Friends', 'No one'],
            onChanged: (v) =>
                setState(() => _whoCanSendMessages = v),
          ),
          _Select(
            title: 'Who can comment',
            value: _whoCanComment,
            options: ['Everyone', 'Friends', 'No one'],
            onChanged: (v) => setState(() => _whoCanComment = v),
          ),
          _Switch(
            title: 'Allow someone to find me by phone/email',
            value: _allowFindByContact,
            onChanged: (v) => setState(() => _allowFindByContact = v),
          ),

          // ── Content ──────────────────────────────────────────────────────
          _Header('Content'),
          _Select(
            title: 'Who can Duet with your videos',
            value: _whoCanDuet,
            options: ['Everyone', 'Friends', 'No one'],
            onChanged: (v) => setState(() => _whoCanDuet = v),
          ),
          _Switch(
            title: 'Allow Stitch',
            subtitle: 'Let others use your videos in their Stitch',
            value: _allowStitch,
            onChanged: (v) => setState(() => _allowStitch = v),
          ),
          _Switch(
            title: 'Allow downloads',
            subtitle: 'Let others download your videos',
            value: _allowDownloads,
            onChanged: (v) => setState(() => _allowDownloads = v),
          ),

          // ── Ads & Data ───────────────────────────────────────────────────
          _Header('Ads & Data'),
          _Switch(
            title: 'Personalised ads',
            subtitle: 'Use your activity to show relevant ads',
            value: _allowAds,
            onChanged: (v) => setState(() => _allowAds = v),
          ),
          _Switch(
            title: 'Suggest your account to others',
            subtitle: 'Let us recommend your account to potential followers',
            value: _allowSuggest,
            onChanged: (v) => setState(() => _allowSuggest = v),
          ),
          _ActionTile(
            title: 'Download your data',
            subtitle: 'Request a copy of your TikTok data',
            onTap: () => ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(
                content: Text('Data request submitted'),
                backgroundColor: Color(0xFF1A1A1A),
                behavior: SnackBarBehavior.floating,
              ),
            ),
          ),

          const SizedBox(height: 32),
        ],
      ),
    );
  }
}

// ── Shared widgets ────────────────────────────────────────────────────────────

class _Header extends StatelessWidget {
  final String title;
  const _Header(this.title);

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

class _Switch extends StatelessWidget {
  final String title;
  final String? subtitle;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _Switch({
    required this.title,
    required this.value,
    required this.onChanged,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    return SwitchListTile(
      title: Text(title,
          style: const TextStyle(color: Colors.white, fontSize: 15)),
      subtitle: subtitle != null
          ? Text(subtitle!,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 12))
          : null,
      value: value,
      onChanged: onChanged,
      activeTrackColor: const Color(0xFFEE1D52),
    );
  }
}

class _Select extends StatelessWidget {
  final String title;
  final String value;
  final List<String> options;
  final ValueChanged<String> onChanged;

  const _Select({
    required this.title,
    required this.value,
    required this.options,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      title: Text(title,
          style: const TextStyle(color: Colors.white, fontSize: 15)),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(value,
              style: const TextStyle(
                  color: Colors.white38, fontSize: 13)),
          const SizedBox(width: 4),
          const Icon(Icons.chevron_right,
              color: Colors.white24, size: 20),
        ],
      ),
      onTap: () => showModalBottomSheet<void>(
        context: context,
        backgroundColor: const Color(0xFF1A1A1A),
        shape: const RoundedRectangleBorder(
          borderRadius:
              BorderRadius.vertical(top: Radius.circular(16)),
        ),
        builder: (_) => SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 12),
              Text(title,
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w700)),
              const SizedBox(height: 8),
              ...options.map((opt) => ListTile(
                    title: Text(opt,
                        style: const TextStyle(
                            color: Colors.white)),
                    trailing: opt == value
                        ? const Icon(Icons.check,
                            color: Color(0xFFEE1D52))
                        : null,
                    onTap: () {
                      onChanged(opt);
                      Navigator.pop(context);
                    },
                  )),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }
}

class _ActionTile extends StatelessWidget {
  final String title;
  final String? subtitle;
  final VoidCallback onTap;

  const _ActionTile({
    required this.title,
    required this.onTap,
    this.subtitle,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
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