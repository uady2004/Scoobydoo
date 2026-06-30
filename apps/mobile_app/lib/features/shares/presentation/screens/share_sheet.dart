import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:share_plus/share_plus.dart';

Future<void> showShareSheet(
  BuildContext context, {
  required String contentUrl,
  String? title,
  String? thumbnailUrl,
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: Colors.transparent,
    builder: (_) => _ShareSheet(
      contentUrl: contentUrl,
      title: title,
      thumbnailUrl: thumbnailUrl,
    ),
  );
}

class _ShareSheet extends StatelessWidget {
  const _ShareSheet({
    required this.contentUrl,
    this.title,
    this.thumbnailUrl,
  });

  final String contentUrl;
  final String? title;
  final String? thumbnailUrl;

  @override
  Widget build(BuildContext context) {
    return SafeArea(                                      // ← ADDED
      top: false,                                         // ← ADDED
      child: Container(
        decoration: const BoxDecoration(
          color: Color(0xFF1A1A1A),
          borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // ── Drag handle ────────────────────────────────────────────────
            Container(
              width: 36,
              height: 4,
              margin: const EdgeInsets.only(top: 12, bottom: 16),
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(2),
              ),
            ),

            // ── Content preview row ────────────────────────────────────────
            if (title != null || thumbnailUrl != null)
              Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
                child: Row(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(6),
                      child: thumbnailUrl != null && thumbnailUrl!.isNotEmpty
                          ? CachedNetworkImage(
                              imageUrl: thumbnailUrl!,
                              width: 48,
                              height: 48,
                              fit: BoxFit.cover,
                              errorWidget: (_, __, ___) => Container(
                                width: 48,
                                height: 48,
                                color: Colors.grey[800],
                              ),
                            )
                          : Container(
                              width: 48,
                              height: 48,
                              color: Colors.grey[800],
                              child: const Icon(Icons.video_library,
                                  color: Colors.white38),
                            ),
                    ),
                    const SizedBox(width: 12),
                    if (title != null)
                      Expanded(
                        child: Text(
                          title!,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 14,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                  ],
                ),
              ),

            const SizedBox(height: 16),

            // ── "Share to" label ───────────────────────────────────────────
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 16),
              child: Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  'Share to',
                  style: TextStyle(color: Colors.white54, fontSize: 12),
                ),
              ),
            ),

            const SizedBox(height: 12),

            // ── Share targets ──────────────────────────────────────────────
            SizedBox(
              height: 96,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                children: [
                  _ShareTarget(
                    icon: Icons.message,
                    label: 'Direct\nMessage',
                    bgColor: const Color(0xFF25F4EE),
                    onTap: () {
                      Navigator.pop(context);
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(
                          content: Text('Open a conversation to share'),
                          behavior: SnackBarBehavior.floating,
                        ),
                      );
                      context.push('/inbox');
                    },
                  ),
                  _ShareTarget(
                    icon: Icons.chat,
                    label: 'WhatsApp',
                    bgColor: const Color(0xFF25D366),
                    onTap: () => _shareText(context, contentUrl),
                  ),
                  _ShareTarget(
                    icon: Icons.camera_alt,
                    label: 'Instagram',
                    bgColor: const Color(0xFFE1306C),
                    onTap: () => _shareText(context, contentUrl),
                  ),
                  _ShareTarget(
                    icon: Icons.close,
                    label: 'Twitter/X',
                    bgColor: Colors.black,
                    onTap: () => _shareText(context, contentUrl),
                  ),
                  _ShareTarget(
                    icon: Icons.thumb_up,
                    label: 'Facebook',
                    bgColor: const Color(0xFF1877F2),
                    onTap: () => _shareText(context, contentUrl),
                  ),
                  _ShareTarget(
                    icon: Icons.link,
                    label: 'Copy Link',
                    bgColor: Colors.grey[700]!,
                    onTap: () async {
                      await Clipboard.setData(
                          ClipboardData(text: contentUrl));
                      if (context.mounted) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(
                            content: Text('Link copied'),
                            behavior: SnackBarBehavior.floating,
                            duration: Duration(seconds: 2),
                          ),
                        );
                      }
                    },
                  ),
                  _ShareTarget(
                    icon: Icons.more_horiz,
                    label: 'More',
                    bgColor: Colors.grey[700]!,
                    onTap: () async {
                      await SharePlus.instance.share(
                        ShareParams(uri: Uri.parse(contentUrl)),
                      );
                    },
                  ),
                ],
              ),
            ),

            const SizedBox(height: 16),

            // ── Not Interested ─────────────────────────────────────────────
            SizedBox(
              width: double.infinity,
              child: TextButton(
                onPressed: () => Navigator.pop(context),
                child: const Text(
                  'Not Interested',
                  style: TextStyle(color: Colors.grey),
                ),
              ),
            ),

            const SizedBox(height: 8),       // ← REPLACED SafeArea shrink
          ],
        ),
      ),
    );                                                    // ← closes SafeArea
  }

  static Future<void> _shareText(
      BuildContext context, String url) async {
    await SharePlus.instance.share(ShareParams(text: url));
  }
}

class _ShareTarget extends StatelessWidget {
  const _ShareTarget({
    required this.icon,
    required this.label,
    required this.bgColor,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final Color bgColor;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 72,
        margin: const EdgeInsets.only(right: 8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: bgColor,
                shape: BoxShape.circle,
              ),
              child: Icon(icon, color: Colors.white, size: 24),
            ),
            const SizedBox(height: 6),
            Text(
              label,
              style: const TextStyle(color: Colors.grey, fontSize: 11),
              textAlign: TextAlign.center,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

class ShareSheet extends StatefulWidget {
  final String videoId;
  final String videoUrl;
  final String? videoTitle;

  const ShareSheet({
    super.key,
    required this.videoId,
    required this.videoUrl,
    this.videoTitle,
  });

  static Future<void> show(
    BuildContext context, {
    required String videoId,
    required String videoUrl,
    String? videoTitle,
  }) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => ShareSheet(
        videoId: videoId,
        videoUrl: videoUrl,
        videoTitle: videoTitle,
      ),
    );
  }

  @override
  State<ShareSheet> createState() => _ShareSheetState();
}

class _ShareSheetState extends State<ShareSheet> {
  bool _showDmSearch = false;
  final _dmSearchController = TextEditingController();

  @override
  void dispose() {
    _dmSearchController.dispose();
    super.dispose();
  }

  String get _shareLink => widget.videoUrl;

  Future<void> _nativeShare() async {
    await SharePlus.instance.share(
      ShareParams(
        text: widget.videoTitle != null
            ? '${widget.videoTitle}\n$_shareLink'
            : _shareLink,
        subject: widget.videoTitle,
      ),
    );
    if (mounted) Navigator.of(context).pop();
  }

  Future<void> _shareToApp(String app) async {
    await SharePlus.instance.share(
      ShareParams(text: 'Check out this video: $_shareLink'),
    );
    if (mounted) Navigator.of(context).pop();
  }

  Future<void> _copyLink() async {
    await Clipboard.setData(ClipboardData(text: _shareLink));
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Link copied!'),
          duration: Duration(seconds: 2),
          behavior: SnackBarBehavior.floating,
        ),
      );
      Navigator.of(context).pop();
    }
  }

  void _showEmbedDialog(BuildContext context) {
    final embedCode =
        '<iframe src="https://tiktok-clone.dev/embed/${widget.videoId}" '
        'width="325" height="576" frameborder="0" allowfullscreen></iframe>';

    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Embed Video',
            style: TextStyle(color: Colors.white)),
        content: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: Colors.white10,
            borderRadius: BorderRadius.circular(8),
          ),
          child: SelectableText(
            embedCode,
            style: const TextStyle(color: Colors.white70, fontSize: 12),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () {
              Clipboard.setData(ClipboardData(text: embedCode));
              Navigator.pop(ctx);
              Navigator.pop(context);
            },
            child: const Text('Copy',
                style: TextStyle(color: Color(0xFFFF0050))),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(                                      // ← FIXED
      top: false,
      child: Container(
        decoration: const BoxDecoration(
          color: Color(0xFF1A1A1A),
          borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 10),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: Colors.white24,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: 16),
            const Text(
              'Share to',
              style: TextStyle(
                color: Colors.white,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 20),
            SizedBox(
              height: 86,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                children: [
                  _ShareOption(
                    icon: Icons.send_rounded,
                    label: 'Direct\nMessage',
                    color: const Color(0xFFFF0050),
                    onTap: () =>
                        setState(() => _showDmSearch = !_showDmSearch),
                  ),
                  _ShareOption(
                    emoji: '\u{1F4AC}',
                    label: 'WhatsApp',
                    color: const Color(0xFF25D366),
                    onTap: () => _shareToApp('whatsapp'),
                  ),
                  _ShareOption(
                    emoji: '\u{1F4F8}',
                    label: 'Instagram',
                    color: const Color(0xFFE1306C),
                    onTap: () => _shareToApp('instagram'),
                  ),
                  _ShareOption(
                    emoji: '\u{1F426}',
                    label: 'Twitter / X',
                    color: Colors.white,
                    onTap: () => _shareToApp('twitter'),
                  ),
                  _ShareOption(
                    icon: Icons.link_rounded,
                    label: 'Copy Link',
                    color: Colors.white70,
                    onTap: _copyLink,
                  ),
                  _ShareOption(
                    icon: Icons.code_rounded,
                    label: 'Embed',
                    color: Colors.white70,
                    onTap: () => _showEmbedDialog(context),
                  ),
                  _ShareOption(
                    icon: Icons.more_horiz_rounded,
                    label: 'More',
                    color: Colors.white70,
                    onTap: _nativeShare,
                  ),
                ],
              ),
            ),
            if (_showDmSearch) ...[
              const Divider(color: Colors.white12, height: 1),
              Padding(
                padding: const EdgeInsets.symmetric(
                    horizontal: 16, vertical: 10),
                child: TextField(
                  controller: _dmSearchController,
                  autofocus: true,
                  style: const TextStyle(color: Colors.white),
                  decoration: InputDecoration(
                    hintText: 'Search users...',
                    hintStyle: const TextStyle(color: Colors.white38),
                    prefixIcon:
                        const Icon(Icons.search, color: Colors.white38),
                    filled: true,
                    fillColor: Colors.white10,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(24),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding:
                        const EdgeInsets.symmetric(vertical: 10),
                  ),
                ),
              ),
              SizedBox(
                height: 130,
                child: ListView.builder(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  itemCount: 3,
                  itemBuilder: (_, i) => ListTile(
                    leading: CircleAvatar(
                      backgroundColor: const Color(0xFF333333),
                      child: Text(
                        'U$i',
                        style:
                            const TextStyle(color: Colors.white70),
                      ),
                    ),
                    title: Text(
                      'user_example_$i',
                      style: const TextStyle(color: Colors.white70),
                    ),
                    trailing: OutlinedButton(
                      onPressed: () {},
                      style: OutlinedButton.styleFrom(
                        foregroundColor: Colors.white,
                        side: const BorderSide(color: Colors.white24),
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14),
                        minimumSize: const Size(0, 32),
                      ),
                      child: const Text('Send'),
                    ),
                  ),
                ),
              ),
            ],
            const SizedBox(height: 12),
          ],
        ),
      ),
    );                                                    // ← closes SafeArea
  }
}

class _ShareOption extends StatelessWidget {
  final IconData? icon;
  final String? emoji;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const _ShareOption({
    this.icon,
    this.emoji,
    required this.label,
    required this.color,
    required this.onTap,
  }) : assert(icon != null || emoji != null,
            'Provide either icon or emoji');

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 72,
        margin: const EdgeInsets.only(right: 12),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 52,
              height: 52,
              decoration: BoxDecoration(
                color: Colors.white10,
                borderRadius: BorderRadius.circular(14),
              ),
              alignment: Alignment.center,
              child: icon != null
                  ? Icon(icon, color: color, size: 26)
                  : Text(emoji!, style: const TextStyle(fontSize: 26)),
            ),
            const SizedBox(height: 4),
            Text(
              label,
              style: const TextStyle(color: Colors.white54, fontSize: 10),
              textAlign: TextAlign.center,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}