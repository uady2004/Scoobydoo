import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class NotificationSettingsScreen extends StatefulWidget {
  const NotificationSettingsScreen({super.key});

  @override
  State<NotificationSettingsScreen> createState() =>
      _NotificationSettingsScreenState();
}

class _NotificationSettingsScreenState
    extends State<NotificationSettingsScreen> {
  // Activity
  bool _likes = true;
  bool _comments = true;
  bool _newFollowers = true;
  bool _mentions = true;
  bool _tags = true;
  // Messages
  bool _directMessages = true;
  bool _groupMessages = true;
  // Live
  bool _liveNotifs = true;
  bool _liveGifts = true;
  // Videos
  bool _videoSuggestions = false;
  bool _friendActivity = true;
  bool _trendingVideos = false;
  // System
  bool _appUpdates = true;
  bool _policyUpdates = false;
  bool _emailNotifs = true;

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
        title: const Text('Notifications',
            style: TextStyle(
                color: Colors.white, fontWeight: FontWeight.w700)),
        centerTitle: true,
        actions: [
          TextButton(
            onPressed: () {
              setState(() {
                _likes = false; _comments = false;
                _newFollowers = false; _mentions = false;
                _tags = false; _directMessages = false;
                _groupMessages = false; _liveNotifs = false;
                _liveGifts = false; _videoSuggestions = false;
                _friendActivity = false; _trendingVideos = false;
                _appUpdates = false; _policyUpdates = false;
                _emailNotifs = false;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text('All notifications turned off'),
                  backgroundColor: Color(0xFF1A1A1A),
                  behavior: SnackBarBehavior.floating,
                ),
              );
            },
            child: const Text('Turn off all',
                style: TextStyle(
                    color: Color(0xFFEE1D52), fontSize: 13)),
          ),
        ],
      ),
      body: ListView(
        children: [
          // ── Activity ─────────────────────────────────────────────────────
          _Header('Activity'),
          _Switch('Likes',
              'When someone likes your video', _likes,
              (v) => setState(() => _likes = v)),
          _Switch('Comments',
              'When someone comments on your video', _comments,
              (v) => setState(() => _comments = v)),
          _Switch('New followers',
              'When someone follows you', _newFollowers,
              (v) => setState(() => _newFollowers = v)),
          _Switch('Mentions',
              'When someone mentions you in a comment',
              _mentions,
              (v) => setState(() => _mentions = v)),
          _Switch('Tags',
              'When someone tags you in a video', _tags,
              (v) => setState(() => _tags = v)),

          // ── Messages ─────────────────────────────────────────────────────
          _Header('Messages'),
          _Switch('Direct messages',
              'New message notifications', _directMessages,
              (v) => setState(() => _directMessages = v)),
          _Switch('Group chat messages', null, _groupMessages,
              (v) => setState(() => _groupMessages = v)),

          // ── Live ─────────────────────────────────────────────────────────
          _Header('Live'),
          _Switch('Live notifications',
              'When someone you follow goes live', _liveNotifs,
              (v) => setState(() => _liveNotifs = v)),
          _Switch('Gift notifications',
              'When you receive gifts in a live stream',
              _liveGifts,
              (v) => setState(() => _liveGifts = v)),

          // ── Videos ───────────────────────────────────────────────────────
          _Header('Videos & Content'),
          _Switch('Video suggestions',
              'Personalised video recommendations',
              _videoSuggestions,
              (v) => setState(() => _videoSuggestions = v)),
          _Switch('Friend activity',
              'When friends post or interact', _friendActivity,
              (v) => setState(() => _friendActivity = v)),
          _Switch('Trending videos', null, _trendingVideos,
              (v) => setState(() => _trendingVideos = v)),

          // ── System ───────────────────────────────────────────────────────
          _Header('System'),
          _Switch('App updates',
              'New features and announcements', _appUpdates,
              (v) => setState(() => _appUpdates = v)),
          _Switch('Policy updates', null, _policyUpdates,
              (v) => setState(() => _policyUpdates = v)),
          _Switch('Email notifications',
              'Receive notifications by email', _emailNotifs,
              (v) => setState(() => _emailNotifs = v)),

          const SizedBox(height: 32),
        ],
      ),
    );
  }
}

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

  const _Switch(this.title, this.subtitle, this.value, this.onChanged);

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