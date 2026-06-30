import 'dart:async';
import 'dart:io';

import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

const _kRed = Color(0xFFEE1D52);
const _kSurface = Color(0xFF1C1C1E);
const _kTeal = Color(0xFF69C9D0);

const List<String> _kSuggestedHashtags = [
  'fyp', 'trending', 'viral', 'tiktok', 'foryoupage', 'xyzbca',
];

// ---------------------------------------------------------------------------
// Upload screen — exact TikTok camera UI
// ---------------------------------------------------------------------------

class UploadScreen extends StatefulWidget {
  const UploadScreen({super.key});

  @override
  State<UploadScreen> createState() => _UploadScreenState();
}

class _UploadScreenState extends State<UploadScreen> {
  // Camera
  List<CameraDescription> _cameras = [];
  CameraController? _ctrl;
  bool _ready = false;
  bool _isFront = false;
  bool _isRecording = false;
  bool _isCountingDown = false;
  int _countdown = 0;
  Timer? _countdownTimer;
  Timer? _recordTimer;
  double _recordSecs = 0;
  FlashMode _flash = FlashMode.off;
  double _zoom = 1.0;
  double _minZoom = 1.0;
  double _maxZoom = 8.0;

  // Mode
  String _duration = 'PHOTO'; // 10m 60s 15s PHOTO TEXT
  String _tab = 'POST';       // POST CREATE LIVE

  // Right toolbar
  int _timerSecs = 0;
  String _speed = '1x';
  String _filter = 'Original';
  double _beautySmooth = 0.3;
  double _beautyWhiten = 0.2;
  double _beautySlim = 0.0;

  // Post flow
  XFile? _captured;
  final _captionCtrl = TextEditingController();
  bool _isUploading = false;
  double _uploadProgress = 0;
  Timer? _uploadTimer;
  bool _isPublic = true;
  bool _allowComments = true;
  bool _allowDuet = true;
  bool _allowStitch = true;
  bool _isScheduled = false;
  final List<String> _hashtags = [];

  bool _toolsExpanded = false;
  int _countdownDuration = 0;
  String _voiceEffect = 'None';
  String _layout = 'Single';
  bool _teleprompterEnabled = false;
  bool _qaEnabled = false;
  bool _greenScreenEnabled = false;
  String _teleprompterText = '';

  @override
  void initState() {
    super.initState();
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _initCamera();
  }

  Future<void> _initCamera() async {
    try {
      _cameras = await availableCameras();
      if (_cameras.isEmpty) return;
      await _setupCamera(_cameras.first);
    } catch (_) {}
  }

  Future<void> _setupCamera(CameraDescription cam) async {
    final c = CameraController(cam, ResolutionPreset.high, enableAudio: true);
    try {
      await c.initialize();
      _minZoom = await c.getMinZoomLevel();
      _maxZoom = await c.getMaxZoomLevel();
      if (mounted) setState(() { _ctrl?.dispose(); _ctrl = c; _ready = true; });
    } catch (_) { c.dispose(); }
  }

  Future<void> _flipCamera() async {
    if (_cameras.length < 2) return;
    _isFront = !_isFront;
    final cam = _cameras.firstWhere(
      (c) => c.lensDirection ==
          (_isFront ? CameraLensDirection.front : CameraLensDirection.back),
      orElse: () => _cameras.first,
    );
    setState(() => _ready = false);
    await _setupCamera(cam);
  }

  Future<void> _cycleFlash() async {
    final modes = [FlashMode.off, FlashMode.auto, FlashMode.always, FlashMode.torch];
    final next = modes[(modes.indexOf(_flash) + 1) % modes.length];
    await _ctrl?.setFlashMode(next);
    setState(() => _flash = next);
  }

  IconData get _flashIcon {
    switch (_flash) {
      case FlashMode.auto: return Icons.flash_auto;
      case FlashMode.always: return Icons.flash_on;
      case FlashMode.torch: return Icons.highlight;
      default: return Icons.flash_off;
    }
  }

  void _onRecordTap() {
    if (_duration == 'PHOTO') { _takePhoto(); return; }
    if (_isCountingDown) { _cancelCountdown(); return; }
    if (_isRecording) { _stopRec(); return; }
    if (_timerSecs > 0) { _startCountdown(); return; }
    _startRec();
  }

  Future<void> _takePhoto() async {
    if (_ctrl == null || !_ready) return;
    try {
      final f = await _ctrl!.takePicture();
      if (mounted) setState(() => _captured = f);
    } catch (_) {}
  }

  void _startCountdown() {
    setState(() { _isCountingDown = true; _countdown = _timerSecs; });
    _countdownTimer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (!mounted) { t.cancel(); return; }
      if (_countdown <= 1) { t.cancel(); setState(() => _isCountingDown = false); _startRec(); }
      else { setState(() => _countdown--); }
    });
  }

  void _cancelCountdown() {
    _countdownTimer?.cancel();
    setState(() { _isCountingDown = false; _countdown = 0; });
  }

  Future<void> _startRec() async {
    if (_ctrl == null || !_ready) return;
    await _ctrl!.startVideoRecording();
    setState(() { _isRecording = true; _recordSecs = 0; });
    _recordTimer = Timer.periodic(const Duration(milliseconds: 100), (_) {
      if (!mounted) return;
      setState(() => _recordSecs += 0.1);
      final lim = _duration == '15s' ? 15.0 : _duration == '60s' ? 60.0 : 600.0;
      if (_recordSecs >= lim) _stopRec();
    });
  }

  Future<void> _stopRec() async {
    _recordTimer?.cancel();
    if (!_isRecording || _ctrl == null) return;
    final f = await _ctrl!.stopVideoRecording();
    setState(() { _isRecording = false; _captured = f; });
  }

  Future<void> _pickGallery() async {
    final f = _duration == 'PHOTO'
        ? await ImagePicker().pickImage(source: ImageSource.gallery)
        : await ImagePicker().pickVideo(source: ImageSource.gallery);
    if (f != null && mounted) setState(() => _captured = f);
  }

  void _onScale(ScaleUpdateDetails d) {
    if (_ctrl == null) return;
    final z = (_zoom * d.scale).clamp(_minZoom, _maxZoom);
    _ctrl!.setZoomLevel(z);
    setState(() => _zoom = z);
  }

  @override
  void dispose() {
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _countdownTimer?.cancel();
    _recordTimer?.cancel();
    _uploadTimer?.cancel();
    _ctrl?.dispose();
    _captionCtrl.dispose();
    super.dispose();
  }

  double get _recLimit =>
      _duration == '15s' ? 15.0 : _duration == '60s' ? 60.0 : 600.0;

  String get _timeStr {
    final s = _recordSecs.toInt();
    return '${(s ~/ 60).toString().padLeft(2, '0')}:${(s % 60).toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    if (_captured != null) return _buildPost();
    return _buildCamera();
  }

  // ---------------------------------------------------------------------------
  // CAMERA SCREEN — exact TikTok layout
  // ---------------------------------------------------------------------------

  Widget _buildCamera() {
    return Scaffold(
      backgroundColor: Colors.black,
      body: GestureDetector(
        onScaleUpdate: _onScale,
        child: Stack(
          fit: StackFit.expand,
          children: [

            // ── Camera preview ──────────────────────────────────────────
            if (_ready && _ctrl != null)
              CameraPreview(_ctrl!)
            else
              const ColoredBox(color: Colors.black),

            // ── Countdown overlay ───────────────────────────────────────
            if (_isCountingDown)
              Container(
                color: Colors.black54,
                alignment: Alignment.center,
                child: Text('$_countdown',
                    style: const TextStyle(
                        color: Colors.white,
                        fontSize: 120,
                        fontWeight: FontWeight.bold)),
              ),

            // ── Top bar: X | Add sound | refresh icon ───────────────────
            Positioned(
              top: MediaQuery.of(context).padding.top + 8,
              left: 12, right: 12,
              child: Row(
                children: [
                  _Btn(icon: Icons.close, onTap: () => context.pop()),
                  const Spacer(),
                  GestureDetector(
                    onTap: _showMusicSheet,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 14, vertical: 7),
                      decoration: BoxDecoration(
                        color: Colors.black54,
                        borderRadius: BorderRadius.circular(20),
                        border: Border.all(color: Colors.white30),
                      ),
                      child: const Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.music_note,
                              color: Colors.white, size: 14),
                          SizedBox(width: 5),
                          Text('Add sound',
                              style: TextStyle(
                                  color: Colors.white, fontSize: 13)),
                        ],
                      ),
                    ),
                  ),
                  const Spacer(),
                  _Btn(
                    icon: Icons.refresh,
                    onTap: _flipCamera,
                  ),
                ],
              ),
            ),

            // ── Right toolbar ───────────────────────────────────────────
            // ── Right toolbar ───────────────────────────────────────────
            Positioned(
              right: 10,
              top: MediaQuery.of(context).padding.top + 64,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _ToolItem(
                    icon: _flashIcon,
                    label: 'Flash',
                    onTap: _cycleFlash,
                    active: _flash != FlashMode.off,
                  ),
                  const SizedBox(height: 20),
                  _ToolItem(
                    icon: Icons.timer_outlined,
                    label: 'Timer',
                    badge: _timerSecs == 0 ? null : '${_timerSecs}s',
                    onTap: _showTimerSheet,
                  ),
                  const SizedBox(height: 20),
                  _ToolItem(
                    icon: Icons.grid_on_outlined,
                    label: 'Layouts',
                    onTap: () => _showLayoutsSheet(),
                  ),
                  const SizedBox(height: 20),
                  _ToolItem(
                    icon: Icons.auto_awesome_outlined,
                    label: 'Effects',
                    badge: _filter == 'Original' ? null : '●',
                    onTap: _showFiltersSheet,
                  ),
                  const SizedBox(height: 20),
                  _ToolItem(
                    icon: Icons.face_retouching_natural_outlined,
                    label: 'Enhance',
                    badge: (_beautySmooth + _beautyWhiten + _beautySlim) > 0.1
                        ? 'ON'
                        : null,
                    onTap: _showBeautySheet,
                  ),
                  const SizedBox(height: 20),
                  // Expanded tools
                  if (_toolsExpanded) ...[
                    _ToolItem(
                      icon: Icons.speed,
                      label: 'Speed',
                      badge: _speed == '1x' ? null : _speed,
                      onTap: _showSpeedSheet,
                    ),
                    const SizedBox(height: 20),
                    _ToolItem(
                      icon: Icons.question_answer_outlined,
                      label: 'Q&A',
                      onTap: _showQASheet,
                    ),
                    const SizedBox(height: 20),
                    _ToolItem(
                      icon: Icons.article_outlined,
                      label: 'Prompter',
                      onTap: _showTeleprompterSheet,
                    ),
                    const SizedBox(height: 20),
                    _ToolItem(
                      icon: Icons.av_timer_outlined,
                      label: 'Countdown',
                      badge: _countdownDuration == 0 ? null : '${_countdownDuration}s',
                      onTap: _showCountdownSheet,
                    ),
                    const SizedBox(height: 20),
                    _ToolItem(
                      icon: Icons.record_voice_over_outlined,
                      label: 'Voice',
                      badge: _voiceEffect == 'None' ? null : _voiceEffect,
                      onTap: _showVoiceEffectsSheet,
                    ),
                    const SizedBox(height: 20),
                    _ToolItem(
                      icon: Icons.filter_b_and_w_outlined,
                      label: 'Green',
                      onTap: _showGreenScreenSheet,
                    ),
                    const SizedBox(height: 20),
                  ],
                  // Expand/collapse arrow
                  GestureDetector(
                    onTap: () =>
                        setState(() => _toolsExpanded = !_toolsExpanded),
                    child: Icon(
                      _toolsExpanded
                          ? Icons.keyboard_arrow_up_rounded
                          : Icons.keyboard_arrow_down_rounded,
                      color: Colors.white,
                      size: 26,
                      shadows: const [
                        Shadow(color: Colors.black, blurRadius: 6)
                      ],
                    ),
                  ),
                ],
              ),
            ),

            // ── Recording timer ─────────────────────────────────────────
            if (_isRecording)
              Positioned(
                top: MediaQuery.of(context).padding.top + 60,
                left: 0, right: 0,
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 12, vertical: 5),
                    decoration: BoxDecoration(
                      color: _kRed,
                      borderRadius: BorderRadius.circular(16),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.fiber_manual_record,
                            color: Colors.white, size: 10),
                        const SizedBox(width: 4),
                        Text(_timeStr,
                            style: const TextStyle(
                                color: Colors.white,
                                fontSize: 13,
                                fontWeight: FontWeight.bold)),
                      ],
                    ),
                  ),
                ),
              ),

            // ── Zoom badge ──────────────────────────────────────────────
            if (_zoom > 1.05)
              Positioned(
                top: MediaQuery.of(context).padding.top + 60,
                left: 0, right: 0,
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                        horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: Colors.black54,
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text('${_zoom.toStringAsFixed(1)}×',
                        style: const TextStyle(
                            color: Colors.white,
                            fontSize: 13,
                            fontWeight: FontWeight.bold)),
                  ),
                ),
              ),

            // ── Bottom area ─────────────────────────────────────────────
            Positioned(
              bottom: 0, left: 0, right: 0,
              child: Container(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.bottomCenter,
                    end: Alignment.topCenter,
                    colors: [
                      Colors.black.withValues(alpha: 0.95),
                      Colors.transparent,
                    ],
                    stops: const [0.0, 1.0],
                  ),
                ),
                child: SafeArea(
                  top: false,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      // Recording progress bar
                      if (_isRecording)
                        Padding(
                          padding: const EdgeInsets.fromLTRB(24, 0, 24, 8),
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(2),
                            child: LinearProgressIndicator(
                              value: (_recordSecs / _recLimit).clamp(0.0, 1.0),
                              backgroundColor: Colors.white24,
                              valueColor: const AlwaysStoppedAnimation(_kRed),
                              minHeight: 3,
                            ),
                          ),
                        ),

                      // Duration selector: 10m 60s 15s PHOTO TEXT
                      _buildDurationRow(),
                      const SizedBox(height: 16),

                      // Gallery | Record button | Recent photos
                      _buildRecordRow(),
                      const SizedBox(height: 16),

                      // POST | CREATE | LIVE tabs
                      _buildTabRow(),
                      const SizedBox(height: 12),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDurationRow() {
    final durations = ['10m', '60s', '15s', 'PHOTO', 'TEXT'];
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: durations.map((d) {
        final sel = d == _duration;
        return GestureDetector(
          onTap: () => setState(() => _duration = d),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                AnimatedDefaultTextStyle(
                  duration: const Duration(milliseconds: 150),
                  style: TextStyle(
                    color: sel ? Colors.white : Colors.white54,
                    fontSize: sel ? 15 : 14,
                    fontWeight:
                        sel ? FontWeight.bold : FontWeight.w400,
                  ),
                  child: Text(d),
                ),
                const SizedBox(height: 3),
                AnimatedContainer(
                  duration: const Duration(milliseconds: 150),
                  height: 2,
                  width: sel ? 20 : 0,
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(1),
                  ),
                ),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildRecordRow() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          // Gallery thumbnail
          GestureDetector(
            onTap: _pickGallery,
            child: Container(
              width: 52,
              height: 52,
              decoration: BoxDecoration(
                color: Colors.white12,
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: Colors.white24),
              ),
              child: const Icon(Icons.photo_library_outlined,
                  color: Colors.white, size: 24),
            ),
          ),

          // Record button — TikTok style
          GestureDetector(
            onTap: _onRecordTap,
            child: _duration == 'PHOTO'
                ? _buildPhotoButton()
                : _buildVideoButton(),
          ),

          // Recent thumbnails row (2 small)
          Row(
            children: [
              _RecentThumb(onTap: _pickGallery),
              const SizedBox(width: 6),
              _RecentThumb(onTap: _pickGallery),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildPhotoButton() {
    return Container(
      width: 80,
      height: 80,
      decoration: const BoxDecoration(
        shape: BoxShape.circle,
        color: Colors.white,
      ),
    );
  }

  Widget _buildVideoButton() {
    return Stack(
      alignment: Alignment.center,
      children: [
        // Outer teal+red ring
        Container(
          width: 88,
          height: 88,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            gradient: const SweepGradient(
              colors: [_kTeal, _kRed, _kTeal],
            ),
          ),
        ),
        // Black ring
        Container(
          width: 80,
          height: 80,
          decoration: const BoxDecoration(
            shape: BoxShape.circle,
            color: Colors.black,
          ),
        ),
        // Inner red circle / stop square
        AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          width: _isRecording ? 30 : 68,
          height: _isRecording ? 30 : 68,
          decoration: BoxDecoration(
            color: _isCountingDown ? Colors.orange : _kRed,
            borderRadius:
                BorderRadius.circular(_isRecording ? 6 : 34),
          ),
        ),
      ],
    );
  }

  Widget _buildTabRow() {
    final tabs = ['POST', 'CREATE', 'LIVE'];
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: tabs.map((t) {
        final sel = t == _tab;
        return GestureDetector(
          onTap: () {
            if (t == 'LIVE') { context.push('/go-live'); return; }
            setState(() => _tab = t);
          },
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: Text(
              t,
              style: TextStyle(
                color: sel ? Colors.white : Colors.white54,
                fontSize: 14,
                fontWeight: sel ? FontWeight.bold : FontWeight.w500,
                letterSpacing: 0.5,
              ),
            ),
          ),
        );
      }).toList(),
    );
  }

  // ---------------------------------------------------------------------------
  // POST DETAILS SCREEN
  // ---------------------------------------------------------------------------

  Widget _buildPost() {
    if (_isUploading) {
      return Scaffold(
        backgroundColor: Colors.black,
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(
                width: 80, height: 80,
                child: CircularProgressIndicator(
                  value: _uploadProgress / 100,
                  strokeWidth: 5,
                  backgroundColor: Colors.white12,
                  valueColor: const AlwaysStoppedAnimation(_kRed),
                ),
              ),
              const SizedBox(height: 20),
              Text('${_uploadProgress.toInt()}%',
                  style: const TextStyle(
                      color: Colors.white,
                      fontSize: 20,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              const Text('Uploading...',
                  style: TextStyle(color: Colors.white54, fontSize: 14)),
            ],
          ),
        ),
      );
    }

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios_new,
              color: Colors.white, size: 18),
          onPressed: () => setState(() => _captured = null),
        ),
        title: const Text('Post',
            style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w700,
                fontSize: 17)),
        centerTitle: true,
        actions: [
          TextButton(
            onPressed: () => _snack('Draft saved'),
            child: const Text('Drafts',
                style: TextStyle(color: Colors.white70, fontSize: 13)),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              padding: const EdgeInsets.symmetric(
                  horizontal: 16, vertical: 12),
              children: [
                // Preview + caption
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: Container(
                        width: 80, height: 106,
                        color: const Color(0xFF2A2A2A),
                        child: const Icon(Icons.play_circle_fill,
                            color: Colors.white38, size: 36),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: TextField(
                        controller: _captionCtrl,
                        maxLines: 5,
                        maxLength: 300,
                        style: const TextStyle(
                            color: Colors.white, fontSize: 15),
                        cursorColor: _kRed,
                        decoration: InputDecoration(
                          hintText: 'Describe your video... #hashtag @mention',
                          hintStyle: TextStyle(
                              color: Colors.white.withValues(alpha: 0.35),
                              fontSize: 14),
                          counterStyle: TextStyle(
                              color: Colors.white.withValues(alpha: 0.4),
                              fontSize: 12),
                          border: InputBorder.none,
                          contentPadding: EdgeInsets.zero,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),

                // Hashtag quick-picks
                SizedBox(
                  height: 34,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: _kSuggestedHashtags.length,
                    separatorBuilder: (_, __) => const SizedBox(width: 8),
                    itemBuilder: (_, i) {
                      final tag = _kSuggestedHashtags[i];
                      final sel = _hashtags.contains(tag);
                      return GestureDetector(
                        onTap: () => setState(() => sel
                            ? _hashtags.remove(tag)
                            : _hashtags.add(tag)),
                        child: AnimatedContainer(
                          duration: const Duration(milliseconds: 150),
                          padding: const EdgeInsets.symmetric(
                              horizontal: 12, vertical: 5),
                          decoration: BoxDecoration(
                            color: sel ? _kRed : _kSurface,
                            borderRadius: BorderRadius.circular(20),
                            border: Border.all(
                                color: sel ? _kRed : Colors.white12),
                          ),
                          child: Text('#$tag',
                              style: TextStyle(
                                  color: sel
                                      ? Colors.white
                                      : Colors.white60,
                                  fontSize: 12)),
                        ),
                      );
                    },
                  ),
                ),
                const SizedBox(height: 4),
                const Divider(color: Colors.white12),

                _Tile(icon: Icons.music_note_outlined, title: 'Add sound',
                    onTap: _showMusicSheet),
                _Tile(icon: Icons.location_on_outlined, title: 'Add location',
                    onTap: () => _snack('Location — coming soon')),
                _Tile(icon: Icons.people_outline, title: 'Tag people',
                    onTap: () => _snack('Tag people — coming soon')),
                _Tile(icon: Icons.local_offer_outlined, title: 'Tag products',
                    onTap: () => _snack('Tag products — coming soon')),
                const Divider(color: Colors.white12),

                // Who can watch
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(Icons.public_outlined,
                      color: Colors.white70),
                  title: const Text('Who can watch',
                      style: TextStyle(color: Colors.white)),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(_isPublic ? 'Everyone' : 'Only me',
                          style: const TextStyle(
                              color: Colors.white38, fontSize: 13)),
                      const Icon(Icons.chevron_right,
                          color: Colors.white24, size: 20),
                    ],
                  ),
                  onTap: _showVisibilitySheet,
                ),
                const Divider(color: Colors.white12),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  secondary: const Icon(Icons.chat_bubble_outline,
                      color: Colors.white70),
                  title: const Text('Allow comments',
                      style: TextStyle(color: Colors.white)),
                  value: _allowComments,
                  activeTrackColor: _kRed,
                  onChanged: (v) => setState(() => _allowComments = v),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  secondary: const Icon(Icons.people_outline,
                      color: Colors.white70),
                  title: const Text('Allow Duet',
                      style: TextStyle(color: Colors.white)),
                  value: _allowDuet,
                  activeTrackColor: _kRed,
                  onChanged: (v) => setState(() => _allowDuet = v),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  secondary: const Icon(Icons.content_cut_outlined,
                      color: Colors.white70),
                  title: const Text('Allow Stitch',
                      style: TextStyle(color: Colors.white)),
                  value: _allowStitch,
                  activeTrackColor: _kRed,
                  onChanged: (v) => setState(() => _allowStitch = v),
                ),
                const Divider(color: Colors.white12),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  secondary: const Icon(Icons.schedule_outlined,
                      color: Colors.white70),
                  title: const Text('Schedule',
                      style: TextStyle(color: Colors.white)),
                  value: _isScheduled,
                  activeTrackColor: _kRed,
                  onChanged: (v) {
                    setState(() => _isScheduled = v);
                    if (v) _showSchedulePicker();
                  },
                ),
                const SizedBox(height: 24),
              ],
            ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
              child: GestureDetector(
                onTap: _startUpload,
                child: Container(
                  height: 52,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(6),
                    gradient: const LinearGradient(
                      colors: [Color(0xFFEE1D52), Color(0xFFFF3B5C)],
                    ),
                  ),
                  alignment: Alignment.center,
                  child: const Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.rocket_launch_outlined,
                          color: Colors.white, size: 18),
                      SizedBox(width: 8),
                      Text('Post',
                          style: TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.w700,
                              fontSize: 16)),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _startUpload() {
    setState(() { _isUploading = true; _uploadProgress = 0; });
    _uploadTimer = Timer.periodic(const Duration(milliseconds: 80), (t) {
      if (!mounted) { t.cancel(); return; }
      if (_uploadProgress < 95) {
        setState(() => _uploadProgress += 2);
      } else {
        t.cancel();
        Future.delayed(const Duration(seconds: 1), () {
          if (!mounted) return;
          _snack('Posted! 🎉');
          context.pop();
        });
      }
    });
  }

  void _showVisibilitySheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 12),
            const Text('Who can watch',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            _SheetTile('Everyone', _isPublic, () {
              setState(() => _isPublic = true);
              Navigator.pop(context);
            }),
            _SheetTile('Friends', false, () {
              setState(() => _isPublic = true);
              Navigator.pop(context);
            }),
            _SheetTile('Only me', !_isPublic, () {
              setState(() => _isPublic = false);
              Navigator.pop(context);
            }),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  void _showSchedulePicker() {
    showDatePicker(
      context: context,
      initialDate: DateTime.now().add(const Duration(days: 1)),
      firstDate: DateTime.now(),
      lastDate: DateTime.now().add(const Duration(days: 10)),
      builder: (ctx, child) => Theme(
        data: ThemeData.dark()
            .copyWith(colorScheme: const ColorScheme.dark(primary: _kRed)),
        child: child!,
      ),
    ).then((d) {
      if (d != null) _snack('Scheduled for ${d.day}/${d.month}/${d.year}');
    });
  }

  // ── Sheets ──────────────────────────────────────────────────────────────────

  void _showMusicSheet() {
    const tracks = [
      ('Trending Sound #1', 'Artist One', '0:30'),
      ('Viral Beat 2024', 'DJ Remix', '0:15'),
      ('Chill Vibes', 'Lo-Fi Studio', '1:00'),
      ('Epic Intro', 'Sound FX', '0:10'),
      ('Dance Mix Vol. 3', 'Top Hits', '0:30'),
      ('Original Sound', 'You', '—'),
    ];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => DraggableScrollableSheet(
        initialChildSize: 0.6,
        minChildSize: 0.4,
        maxChildSize: 0.9,
        expand: false,
        builder: (_, sc) => Column(
          children: [
            const SizedBox(height: 12),
            _Handle(),
            const SizedBox(height: 12),
            const Text('Add Sound',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.bold)),
            const SizedBox(height: 12),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Container(
                height: 40,
                decoration: BoxDecoration(
                  color: Colors.white10,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: const Row(children: [
                  SizedBox(width: 12),
                  Icon(Icons.search, color: Colors.white38, size: 20),
                  SizedBox(width: 8),
                  Text('Search sounds...',
                      style: TextStyle(color: Colors.white38, fontSize: 14)),
                ]),
              ),
            ),
            const SizedBox(height: 8),
            Expanded(
              child: ListView.builder(
                controller: sc,
                itemCount: tracks.length,
                itemBuilder: (_, i) {
                  final (title, artist, dur) = tracks[i];
                  return ListTile(
                    leading: Container(
                      width: 44, height: 44,
                      decoration: BoxDecoration(
                        color: _kRed.withValues(alpha: 0.2),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: const Icon(Icons.music_note,
                          color: _kRed, size: 22),
                    ),
                    title: Text(title,
                        style: const TextStyle(
                            color: Colors.white, fontSize: 14)),
                    subtitle: Text('$artist • $dur',
                        style: const TextStyle(
                            color: Colors.white54, fontSize: 12)),
                    trailing: GestureDetector(
                      onTap: () => Navigator.pop(ctx),
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 14, vertical: 7),
                        decoration: BoxDecoration(
                          color: _kRed,
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: const Text('Use',
                            style: TextStyle(
                                color: Colors.white,
                                fontSize: 12,
                                fontWeight: FontWeight.bold)),
                      ),
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _showTimerSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Timer',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              const Text('Countdown before recording starts',
                  style: TextStyle(color: Colors.white54, fontSize: 13)),
              const SizedBox(height: 24),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [0, 3, 10].map((s) {
                  final sel = _timerSecs == s;
                  return GestureDetector(
                    onTap: () {
                      setState(() => _timerSecs = s);
                      Navigator.pop(ctx);
                    },
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 200),
                      padding: const EdgeInsets.symmetric(
                          horizontal: 28, vertical: 14),
                      decoration: BoxDecoration(
                        color: sel ? _kRed : Colors.white10,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                            color: sel ? Colors.transparent : Colors.white12),
                      ),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            s == 0
                                ? Icons.timer_off_outlined
                                : Icons.timer_outlined,
                            color: sel ? Colors.white : Colors.white54,
                            size: 22,
                          ),
                          const SizedBox(height: 4),
                          Text(s == 0 ? 'Off' : '${s}s',
                              style: TextStyle(
                                  color: sel ? Colors.white : Colors.white70,
                                  fontWeight: sel
                                      ? FontWeight.bold
                                      : FontWeight.w500,
                                  fontSize: 15)),
                        ],
                      ),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  void _showFiltersSheet() {
    const filters = [
      ('Original', Color(0xFF888888)),
      ('Warm', Color(0xFFFF8C00)),
      ('Cool', Color(0xFF1E90FF)),
      ('Vintage', Color(0xFF8B7355)),
      ('Vivid', Color(0xFFFF1493)),
      ('Fade', Color(0xFF87CEEB)),
      ('Noir', Color(0xFF444444)),
      ('Neon', Color(0xFF39FF14)),
    ];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, ss) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 20, 16, 16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _Handle(),
                const SizedBox(height: 12),
                const Text('Filters',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.bold)),
                const SizedBox(height: 20),
                SizedBox(
                  height: 96,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    itemCount: filters.length,
                    separatorBuilder: (_, __) => const SizedBox(width: 12),
                    itemBuilder: (_, i) {
                      final (name, color) = filters[i];
                      final sel = _filter == name;
                      return GestureDetector(
                        onTap: () {
                          setState(() => _filter = name);
                          ss(() {});
                          Navigator.pop(ctx);
                        },
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            AnimatedContainer(
                              duration: const Duration(milliseconds: 150),
                              width: 64, height: 64,
                              decoration: BoxDecoration(
                                borderRadius: BorderRadius.circular(12),
                                color: color,
                                border: Border.all(
                                  color: sel ? Colors.white : Colors.transparent,
                                  width: 2.5,
                                ),
                              ),
                              child: sel
                                  ? const Icon(Icons.check,
                                      color: Colors.white, size: 28)
                                  : null,
                            ),
                            const SizedBox(height: 5),
                            Text(name,
                                style: TextStyle(
                                    color: sel ? Colors.white : Colors.white54,
                                    fontSize: 11)),
                          ],
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showBeautySheet() {
    double sm = _beautySmooth, wh = _beautyWhiten, sl = _beautySlim;
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, ss) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(24, 20, 24, 16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _Handle(),
                const SizedBox(height: 12),
                const Text('Beauty',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.bold)),
                const SizedBox(height: 20),
                _Slider('Smooth', Icons.blur_on, sm,
                    (v) => ss(() => sm = v)),
                const SizedBox(height: 12),
                _Slider('Whiten', Icons.wb_sunny_outlined, wh,
                    (v) => ss(() => wh = v)),
                const SizedBox(height: 12),
                _Slider('Slim Face', Icons.face_outlined, sl,
                    (v) => ss(() => sl = v)),
                const SizedBox(height: 20),
                Row(children: [
                  Expanded(
                    child: TextButton(
                      onPressed: () => ss(() { sm = 0; wh = 0; sl = 0; }),
                      child: const Text('Reset',
                          style: TextStyle(color: Colors.white54)),
                    ),
                  ),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: () {
                        setState(() {
                          _beautySmooth = sm;
                          _beautyWhiten = wh;
                          _beautySlim = sl;
                        });
                        Navigator.pop(ctx);
                      },
                      style: ElevatedButton.styleFrom(
                        backgroundColor: _kRed,
                        shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(10)),
                      ),
                      child: const Text('Apply',
                          style: TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.bold)),
                    ),
                  ),
                ]),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showSpeedSheet() {
    const speeds = ['0.3x', '0.5x', '1x', '2x', '3x'];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Speed',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 20),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: speeds.map((s) {
                  final sel = _speed == s;
                  return GestureDetector(
                    onTap: () {
                      setState(() => _speed = s);
                      Navigator.pop(ctx);
                    },
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 56, height: 56,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            color: sel ? _kRed : Colors.white10,
                            border: Border.all(
                                color: sel ? Colors.transparent : Colors.white24),
                          ),
                          child: Center(
                            child: Text(s,
                                style: TextStyle(
                                    color: sel ? Colors.white : Colors.white70,
                                    fontWeight: sel
                                        ? FontWeight.bold
                                        : FontWeight.w500,
                                    fontSize: 12)),
                          ),
                        ),
                        const SizedBox(height: 5),
                        Text(
                          s.contains('0.') ? 'Slow' : s == '1x' ? 'Normal' : 'Fast',
                          style: TextStyle(
                              color: sel ? _kRed : Colors.white38,
                              fontSize: 10),
                        ),
                      ],
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }
void _showLayoutsSheet() {
    const layouts = [
      ('Single', Icons.crop_square_outlined),
      ('Split', Icons.view_agenda_outlined),
      ('Grid 2x2', Icons.grid_view_outlined),
      ('Triptych', Icons.view_column_outlined),
    ];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Layouts',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 20),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: layouts.map((l) {
                  final (name, icon) = l;
                  final sel = _layout == name;
                  return GestureDetector(
                    onTap: () {
                      setState(() => _layout = name);
                      Navigator.pop(ctx);
                    },
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        AnimatedContainer(
                          duration: const Duration(milliseconds: 150),
                          width: 64, height: 64,
                          decoration: BoxDecoration(
                            color: sel
                                ? _kRed.withValues(alpha: 0.2)
                                : Colors.white10,
                            borderRadius: BorderRadius.circular(12),
                            border: Border.all(
                              color: sel ? _kRed : Colors.white24,
                              width: sel ? 2 : 1,
                            ),
                          ),
                          child: Icon(icon,
                              color: sel ? _kRed : Colors.white54,
                              size: 28),
                        ),
                        const SizedBox(height: 6),
                        Text(name,
                            style: TextStyle(
                                color: sel ? Colors.white : Colors.white54,
                                fontSize: 11,
                                fontWeight: sel
                                    ? FontWeight.bold
                                    : FontWeight.w400)),
                      ],
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  void _showQASheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(
              20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Q&A',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              const Text(
                'Let viewers ask questions that appear on your screen while recording.',
                style: TextStyle(color: Colors.white54, fontSize: 13),
              ),
              const SizedBox(height: 20),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Enable Q&A',
                    style: TextStyle(color: Colors.white)),
                subtitle: const Text('Viewers can submit questions',
                    style: TextStyle(color: Colors.white38, fontSize: 12)),
                value: _qaEnabled,
                activeTrackColor: _kRed,
                onChanged: (v) {
                  setState(() => _qaEnabled = v);
                  Navigator.pop(ctx);
                  _snack(_qaEnabled ? 'Q&A enabled' : 'Q&A disabled');
                },
              ),
              if (_qaEnabled) ...[
                const Divider(color: Colors.white12),
                const Text('Sample questions:',
                    style: TextStyle(color: Colors.white54, fontSize: 12)),
                const SizedBox(height: 8),
                _QAQuestion('What camera do you use?'),
                _QAQuestion('How long did this take?'),
                _QAQuestion('Can you do a tutorial?'),
              ],
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  void _showTeleprompterSheet() {
    final ctrl = TextEditingController(text: _teleprompterText);
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => Padding(
        padding: EdgeInsets.fromLTRB(
            20, 20, 20, MediaQuery.of(ctx).viewInsets.bottom + 16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _Handle(),
            const SizedBox(height: 12),
            const Text('Teleprompter',
                style: TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.bold)),
            const SizedBox(height: 8),
            const Text(
              'Your script scrolls on screen while you record.',
              style: TextStyle(color: Colors.white54, fontSize: 13),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: ctrl,
              maxLines: 6,
              autofocus: true,
              style: const TextStyle(color: Colors.white, fontSize: 14),
              decoration: InputDecoration(
                hintText: 'Type your script here...',
                hintStyle: const TextStyle(color: Colors.white38),
                filled: true,
                fillColor: Colors.white10,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(ctx),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: Colors.white54,
                      side: const BorderSide(color: Colors.white24),
                    ),
                    child: const Text('Cancel'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: ElevatedButton(
                    onPressed: () {
                      setState(() {
                        _teleprompterText = ctrl.text;
                        _teleprompterEnabled = ctrl.text.isNotEmpty;
                      });
                      Navigator.pop(ctx);
                      _snack(_teleprompterEnabled
                          ? 'Teleprompter ready'
                          : 'Teleprompter cleared');
                    },
                    style: ElevatedButton.styleFrom(
                      backgroundColor: _kRed,
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8)),
                    ),
                    child: const Text('Use script',
                        style: TextStyle(color: Colors.white)),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCountdownSheet() {
    const options = [0, 3, 5, 10, 15, 20, 30, 60];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 20, 20, 16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Countdown',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              const Text(
                'Auto-stop recording after this duration',
                style: TextStyle(color: Colors.white54, fontSize: 13),
              ),
              const SizedBox(height: 20),
              Wrap(
                spacing: 10,
                runSpacing: 10,
                alignment: WrapAlignment.center,
                children: options.map((s) {
                  final sel = _countdownDuration == s;
                  return GestureDetector(
                    onTap: () {
                      setState(() => _countdownDuration = s);
                      Navigator.pop(ctx);
                      _snack(s == 0
                          ? 'Auto-stop disabled'
                          : 'Auto-stop at ${s}s');
                    },
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 150),
                      width: 72, height: 72,
                      decoration: BoxDecoration(
                        color: sel ? _kRed : Colors.white10,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: sel ? Colors.transparent : Colors.white12,
                        ),
                      ),
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Text(
                            s == 0 ? 'Off' : '${s}s',
                            style: TextStyle(
                                color: sel ? Colors.white : Colors.white70,
                                fontSize: 16,
                                fontWeight: sel
                                    ? FontWeight.bold
                                    : FontWeight.w500),
                          ),
                          if (s > 0)
                            Text(
                              s < 60 ? 'sec' : 'min',
                              style: TextStyle(
                                  color: sel
                                      ? Colors.white70
                                      : Colors.white38,
                                  fontSize: 10),
                            ),
                        ],
                      ),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: 8),
            ],
          ),
        ),
      ),
    );
  }

  void _showVoiceEffectsSheet() {
    const effects = [
      ('None', Icons.mic_outlined, Color(0xFF888888)),
      ('Chipmunk', Icons.sentiment_very_satisfied, Color(0xFFFF9800)),
      ('Deep', Icons.record_voice_over, Color(0xFF2196F3)),
      ('Robot', Icons.smart_toy_outlined, Color(0xFF9C27B0)),
      ('Echo', Icons.surround_sound_outlined, Color(0xFF4CAF50)),
      ('Megaphone', Icons.campaign_outlined, Color(0xFFF44336)),
      ('Chorus', Icons.queue_music_outlined, Color(0xFFE91E63)),
      ('Monster', Icons.face_outlined, Color(0xFF795548)),
    ];
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => StatefulBuilder(
        builder: (_, ss) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 20, 16, 16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _Handle(),
                const SizedBox(height: 12),
                const Text('Voice Effects',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.bold)),
                const SizedBox(height: 8),
                const Text(
                  'Change how your voice sounds while recording',
                  style: TextStyle(color: Colors.white54, fontSize: 13),
                ),
                const SizedBox(height: 20),
                GridView.count(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  crossAxisCount: 4,
                  crossAxisSpacing: 10,
                  mainAxisSpacing: 10,
                  childAspectRatio: 0.85,
                  children: effects.map((e) {
                    final (name, icon, color) = e;
                    final sel = _voiceEffect == name;
                    return GestureDetector(
                      onTap: () {
                        setState(() => _voiceEffect = name);
                        ss(() {});
                        Navigator.pop(ctx);
                        _snack(name == 'None'
                            ? 'Voice effect removed'
                            : '$name effect applied');
                      },
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          AnimatedContainer(
                            duration: const Duration(milliseconds: 150),
                            width: 60, height: 60,
                            decoration: BoxDecoration(
                              color: sel
                                  ? color.withValues(alpha: 0.3)
                                  : Colors.white10,
                              borderRadius: BorderRadius.circular(14),
                              border: Border.all(
                                color: sel ? color : Colors.white12,
                                width: sel ? 2 : 1,
                              ),
                            ),
                            child: Icon(icon,
                                color: sel ? color : Colors.white54,
                                size: 26),
                          ),
                          const SizedBox(height: 4),
                          Text(name,
                              style: TextStyle(
                                  color: sel ? Colors.white : Colors.white54,
                                  fontSize: 10,
                                  fontWeight: sel
                                      ? FontWeight.bold
                                      : FontWeight.w400),
                              overflow: TextOverflow.ellipsis),
                        ],
                      ),
                    );
                  }).toList(),
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showGreenScreenSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _Handle(),
              const SizedBox(height: 12),
              const Text('Green Screen',
                  style: TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.bold)),
              const SizedBox(height: 8),
              const Text(
                'Replace your background with any image or video from your gallery.',
                style: TextStyle(color: Colors.white54, fontSize: 13),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  _GreenScreenOption(
                    icon: Icons.image_outlined,
                    label: 'Photo',
                    color: const Color(0xFF2196F3),
                    onTap: () {
                      setState(() => _greenScreenEnabled = true);
                      Navigator.pop(ctx);
                      _snack('Select a photo background');
                    },
                  ),
                  _GreenScreenOption(
                    icon: Icons.videocam_outlined,
                    label: 'Video',
                    color: const Color(0xFF9C27B0),
                    onTap: () {
                      setState(() => _greenScreenEnabled = true);
                      Navigator.pop(ctx);
                      _snack('Select a video background');
                    },
                  ),
                  _GreenScreenOption(
                    icon: Icons.auto_awesome_outlined,
                    label: 'AI Background',
                    color: _kRed,
                    onTap: () {
                      setState(() => _greenScreenEnabled = true);
                      Navigator.pop(ctx);
                      _snack('AI background enabled');
                    },
                  ),
                ],
              ),
              const SizedBox(height: 16),
              if (_greenScreenEnabled)
                TextButton(
                  onPressed: () {
                    setState(() => _greenScreenEnabled = false);
                    Navigator.pop(ctx);
                    _snack('Green screen removed');
                  },
                  child: const Text('Remove background effect',
                      style: TextStyle(color: Colors.white54)),
                ),
            ],
          ),
        ),
      ),
    );
  }
  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg, style: const TextStyle(color: Colors.white)),
      backgroundColor: _kSurface,
      behavior: SnackBarBehavior.floating,
    ));
  }
}

// ---------------------------------------------------------------------------
// Small reusable widgets
// ---------------------------------------------------------------------------

class _Btn extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  const _Btn({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) => GestureDetector(
        onTap: onTap,
        child: Container(
          width: 38, height: 38,
          decoration: const BoxDecoration(
            color: Colors.black45, shape: BoxShape.circle),
          child: Icon(icon, color: Colors.white, size: 20),
        ),
      );
}

class _ToolItem extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  final bool active;
  final String? badge;
  final String? label;

  const _ToolItem({
    required this.icon,
    required this.onTap,
    this.active = false,
    this.badge,
    this.label,
  });

  @override
  Widget build(BuildContext context) => GestureDetector(
        onTap: onTap,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Stack(
              clipBehavior: Clip.none,
              children: [
                Icon(
                  icon,
                  color: active ? _kRed : Colors.white,
                  size: 26,
                  shadows: const [
                    Shadow(color: Colors.black, blurRadius: 6)
                  ],
                ),
                if (badge != null)
                  Positioned(
                    top: -6,
                    right: -10,
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 4, vertical: 1),
                      decoration: BoxDecoration(
                        color: _kRed,
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        badge!,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 8,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            if (label != null) ...[
              const SizedBox(height: 3),
              Text(
                label!,
                style: const TextStyle(
                  color: Colors.white70,
                  fontSize: 10,
                  shadows: [Shadow(color: Colors.black, blurRadius: 4)],
                ),
              ),
            ],
          ],
        ),
      );
}

class _RecentThumb extends StatelessWidget {
  final VoidCallback onTap;
  const _RecentThumb({required this.onTap});

  @override
  Widget build(BuildContext context) => GestureDetector(
        onTap: onTap,
        child: Container(
          width: 40, height: 52,
          decoration: BoxDecoration(
            color: Colors.white12,
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: Colors.white24),
          ),
          child: const Icon(Icons.image_outlined,
              color: Colors.white24, size: 18),
        ),
      );
}

class _Handle extends StatelessWidget {
  @override
  Widget build(BuildContext context) => Container(
        width: 36, height: 4,
        decoration: BoxDecoration(
          color: Colors.white24,
          borderRadius: BorderRadius.circular(2),
        ),
      );
}

class _Tile extends StatelessWidget {
  final IconData icon;
  final String title;
  final VoidCallback onTap;
  const _Tile({required this.icon, required this.title, required this.onTap});

  @override
  Widget build(BuildContext context) => ListTile(
        contentPadding: EdgeInsets.zero,
        leading: Icon(icon, color: Colors.white70, size: 22),
        title: Text(title,
            style: const TextStyle(color: Colors.white, fontSize: 15)),
        trailing: const Icon(Icons.chevron_right,
            color: Colors.white24, size: 20),
        onTap: onTap,
      );
}

class _SheetTile extends StatelessWidget {
  final String title;
  final bool selected;
  final VoidCallback onTap;
  const _SheetTile(this.title, this.selected, this.onTap);

  @override
  Widget build(BuildContext context) => ListTile(
        title: Text(title,
            style: const TextStyle(color: Colors.white)),
        trailing: selected
            ? const Icon(Icons.check, color: _kRed)
            : null,
        onTap: onTap,
      );
}

class _Slider extends StatelessWidget {
  final String label;
  final IconData icon;
  final double value;
  final ValueChanged<double> onChanged;
  const _Slider(this.label, this.icon, this.value, this.onChanged);

  @override
  Widget build(BuildContext context) => Row(
        children: [
          Icon(icon, color: Colors.white54, size: 20),
          const SizedBox(width: 10),
          SizedBox(
            width: 72,
            child: Text(label,
                style: const TextStyle(
                    color: Colors.white70, fontSize: 13)),
          ),
          Expanded(
            child: SliderTheme(
              data: SliderThemeData(
                activeTrackColor: _kRed,
                inactiveTrackColor: Colors.white12,
                thumbColor: Colors.white,
                overlayColor: _kRed.withValues(alpha: 0.2),
                thumbShape: const RoundSliderThumbShape(
                    enabledThumbRadius: 8),
                trackHeight: 3,
              ),
              child: Slider(value: value, onChanged: onChanged),
            ),
          ),
          SizedBox(
            width: 30,
            child: Text('${(value * 100).toInt()}',
                style: const TextStyle(
                    color: Colors.white54, fontSize: 11),
                textAlign: TextAlign.end),
          ),
        ],
      );
}

class _QAQuestion extends StatelessWidget {
  final String text;
  const _QAQuestion(this.text);

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: Colors.white10,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const Icon(Icons.question_mark, color: _kRed, size: 14),
          const SizedBox(width: 8),
          Text(text,
              style: const TextStyle(color: Colors.white70, fontSize: 13)),
        ],
      ),
    );
  }
}

class _GreenScreenOption extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  const _GreenScreenOption({
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 64, height: 64,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.15),
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: color.withValues(alpha: 0.4)),
            ),
            child: Icon(icon, color: color, size: 28),
          ),
          const SizedBox(height: 6),
          Text(label,
              style: const TextStyle(color: Colors.white70, fontSize: 11),
              textAlign: TextAlign.center),
        ],
      ),
    );
  }
}