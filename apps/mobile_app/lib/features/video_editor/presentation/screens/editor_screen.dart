import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';
import 'package:tiktok_clone/features/upload_video/presentation/widgets/video_trimmer.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Data model
// ─────────────────────────────────────────────────────────────────────────────

class _TextItem {
  _TextItem({
    required this.id,
    required this.text,
    required this.x,
    required this.y,
    required this.color,
    required this.fontSize,
  });

  final String id;
  String text;
  double x;
  double y;
  Color color;
  double fontSize;
}

// ─────────────────────────────────────────────────────────────────────────────
// EditorScreen
// ─────────────────────────────────────────────────────────────────────────────

class EditorScreen extends StatefulWidget {
  const EditorScreen({super.key, required this.videoPath});

  final String videoPath;

  @override
  State<EditorScreen> createState() => _EditorScreenState();
}

class _EditorScreenState extends State<EditorScreen> {
  // ── State ──────────────────────────────────────────────────────────────────

  int _activeTab = 0;
  String _selectedFilter = 'None';
  double _selectedSpeed = 1.0;
  final List<_TextItem> _textOverlays = [];
  final TextEditingController _textCtrl = TextEditingController();
  Color _pickedColor = Colors.white;
  double _fontSize = 24.0;

  static const _kRed = Color(0xFFEE1D52);

  static const List<String> _filters = [
    'None', 'Warm', 'Cool', 'B&W', 'Vivid', 'Fade', 'Rose', 'Night',
  ];

  static const List<double> _speeds = [0.3, 0.5, 1.0, 2.0, 3.0];

  static const List<Color> _palette = [
    Colors.white,
    Colors.black,
    Color(0xFFEE1D52),
    Color(0xFFFF9900),
    Color(0xFFFFE600),
    Color(0xFF00C846),
    Color(0xFF00B8FF),
    Color(0xFF6A35FF),
    Color(0xFFFF69B4),
    Color(0xFF8B4513),
  ];

  @override
  void dispose() {
    _textCtrl.dispose();
    super.dispose();
  }

  // ── Helpers ────────────────────────────────────────────────────────────────

  void _addTextOverlay() {
    final text = _textCtrl.text.trim();
    if (text.isEmpty) return;
    final size = MediaQuery.of(context).size;
    setState(() {
      _textOverlays.add(
        _TextItem(
          id: const Uuid().v4(),
          text: text,
          x: size.width * 0.3,
          y: size.height * 0.3,
          color: _pickedColor,
          fontSize: _fontSize,
        ),
      );
      _textCtrl.clear();
    });
  }

  void _editOverlay(_TextItem item) {
    final editCtrl = TextEditingController(text: item.text);
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1A1A1A),
        title: const Text('Edit text', style: TextStyle(color: Colors.white)),
        content: TextField(
          controller: editCtrl,
          autofocus: true,
          style: const TextStyle(color: Colors.white),
          decoration: const InputDecoration(
            enabledBorder: UnderlineInputBorder(
              borderSide: BorderSide(color: Colors.white38),
            ),
            focusedBorder: UnderlineInputBorder(
              borderSide: BorderSide(color: _kRed),
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () {
              setState(() {
                _textOverlays.remove(item);
              });
              Navigator.pop(ctx);
            },
            child: const Text('Remove', style: TextStyle(color: Colors.red)),
          ),
          TextButton(
            onPressed: () {
              setState(() {
                item.text = editCtrl.text.trim().isNotEmpty
                    ? editCtrl.text.trim()
                    : item.text;
              });
              Navigator.pop(ctx);
            },
            child: const Text('Save', style: TextStyle(color: _kRed)),
          ),
        ],
      ),
    );
  }

  Color _filterTint(String filter) {
    switch (filter) {
      case 'Warm':
        return Colors.orange.withValues(alpha: 0.3);
      case 'Cool':
        return Colors.blue.withValues(alpha: 0.3);
      case 'B&W':
        return Colors.grey.withValues(alpha: 0.6);
      case 'Vivid':
        return Colors.purple.withValues(alpha: 0.25);
      case 'Fade':
        return Colors.white.withValues(alpha: 0.2);
      case 'Rose':
        return Colors.pink.withValues(alpha: 0.3);
      case 'Night':
        return Colors.indigo.withValues(alpha: 0.5);
      default:
        return Colors.transparent;
    }
  }

  // ── Tab panels ─────────────────────────────────────────────────────────────

  Widget _buildTextTab() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _textCtrl,
                  style: const TextStyle(color: Colors.white),
                  decoration: const InputDecoration(
                    hintText: 'Add text...',
                    hintStyle: TextStyle(color: Colors.white38),
                    enabledBorder: UnderlineInputBorder(
                      borderSide: BorderSide(color: Colors.white24),
                    ),
                    focusedBorder: UnderlineInputBorder(
                      borderSide: BorderSide(color: _kRed),
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              IconButton(
                onPressed: _addTextOverlay,
                icon: const Icon(Icons.add_circle, color: _kRed, size: 28),
                tooltip: 'Add',
              ),
            ],
          ),
          const SizedBox(height: 6),
          // Color picker row
          SizedBox(
            height: 28,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: _palette.length,
              separatorBuilder: (_, __) => const SizedBox(width: 6),
              itemBuilder: (_, i) {
                final c = _palette[i];
                final selected = _pickedColor == c;
                return GestureDetector(
                  onTap: () => setState(() => _pickedColor = c),
                  child: Container(
                    width: 24,
                    height: 24,
                    decoration: BoxDecoration(
                      color: c,
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: selected ? _kRed : Colors.white38,
                        width: selected ? 2.5 : 1,
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
          const SizedBox(height: 4),
          // Font size slider
          Row(
            children: [
              const Icon(Icons.text_fields, color: Colors.white54, size: 16),
              Expanded(
                child: Slider(
                  value: _fontSize,
                  min: 12,
                  max: 48,
                  activeColor: _kRed,
                  inactiveColor: Colors.white24,
                  onChanged: (v) => setState(() => _fontSize = v),
                ),
              ),
              Text(
                _fontSize.round().toString(),
                style: const TextStyle(color: Colors.white54, fontSize: 12),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildFilterTab() {
    return ListView.builder(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      itemCount: _filters.length,
      itemBuilder: (_, i) {
        final name = _filters[i];
        final selected = _selectedFilter == name;
        return GestureDetector(
          onTap: () => setState(() => _selectedFilter = name),
          child: Container(
            margin: const EdgeInsets.only(right: 10),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 50,
                  height: 70,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: selected ? _kRed : Colors.transparent,
                      width: 2,
                    ),
                    color: Colors.grey[800],
                  ),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(6),
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        Container(color: Colors.grey[700]),
                        Container(color: _filterTint(name)),
                        if (name == 'B&W')
                          Container(
                            color: Colors.black.withValues(alpha: 0.45),
                          ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  name,
                  style: TextStyle(
                    color: selected ? _kRed : Colors.grey,
                    fontSize: 10,
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildSpeedTab() {
    return Center(
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceEvenly,
        children: _speeds.map((s) {
          final selected = _selectedSpeed == s;
          return GestureDetector(
            onTap: () => setState(() => _selectedSpeed = s),
            child: Container(
              padding:
                  const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: selected ? _kRed : Colors.white24,
                  width: 1.5,
                ),
                color: selected
                    ? _kRed.withValues(alpha: 0.15)
                    : Colors.transparent,
              ),
              child: Text(
                '${s}x',
                style: TextStyle(
                  color: selected ? _kRed : Colors.white,
                  fontWeight:
                      selected ? FontWeight.bold : FontWeight.normal,
                  fontSize: 14,
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildTrimTab() {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 16),
      child: VideoTrimmer(
        totalDurationSeconds: 30,
        onTrimChanged: (start, end) {
          // Trim range handled by VideoTrimmer internally;
          // expose via state if needed.
        },
      ),
    );
  }

  Widget _buildTabPanel() {
    switch (_activeTab) {
      case 0:
        return _buildTextTab();
      case 1:
        return _buildFilterTab();
      case 2:
        return _buildSpeedTab();
      case 3:
        return _buildTrimTab();
      default:
        return const SizedBox.shrink();
    }
  }

  // ── Build ──────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        iconTheme: const IconThemeData(color: Colors.white),
        title: const Text(
          'Edit',
          style: TextStyle(color: Colors.white, fontSize: 16),
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(context, {
                'overlays':
                    _textOverlays.map((t) => t.text).toList(),
                'filter': _selectedFilter,
                'speed': _selectedSpeed,
              });
            },
            child: const Text(
              'Next',
              style: TextStyle(
                color: _kRed,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ],
      ),
      body: Column(
        children: [
          // ── Video preview ──────────────────────────────────────────────────
          Expanded(
            child: Stack(
              children: [
                // Grey preview placeholder
                Container(
                  width: double.infinity,
                  height: double.infinity,
                  color: const Color(0xFF2A2A2A),
                  child: const Center(
                    child: Icon(
                      Icons.play_circle_outline,
                      color: Colors.white54,
                      size: 64,
                    ),
                  ),
                ),

                // Text overlays
                for (final item in _textOverlays)
                  Positioned(
                    left: item.x,
                    top: item.y,
                    child: GestureDetector(
                      onPanUpdate: (details) {
                        setState(() {
                          item.x += details.delta.dx;
                          item.y += details.delta.dy;
                        });
                      },
                      onTap: () => _editOverlay(item),
                      child: Text(
                        item.text,
                        style: TextStyle(
                          color: item.color,
                          fontSize: item.fontSize,
                          shadows: const [
                            Shadow(
                              blurRadius: 4,
                              color: Colors.black87,
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),

          // ── Bottom editor panel ────────────────────────────────────────────
          Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Tab bar
              Container(
                color: Colors.black,
                child: Row(
                  children: [
                    for (int i = 0; i < 4; i++)
                      Expanded(
                        child: GestureDetector(
                          onTap: () => setState(() => _activeTab = i),
                          child: Container(
                            padding:
                                const EdgeInsets.symmetric(vertical: 10),
                            decoration: BoxDecoration(
                              border: Border(
                                bottom: BorderSide(
                                  color: _activeTab == i
                                      ? _kRed
                                      : Colors.transparent,
                                  width: 2,
                                ),
                              ),
                            ),
                            child: Text(
                              const ['Text', 'Filter', 'Speed', 'Trim'][i],
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: _activeTab == i
                                    ? Colors.white
                                    : Colors.grey,
                                fontSize: 13,
                                fontWeight: _activeTab == i
                                    ? FontWeight.w600
                                    : FontWeight.normal,
                              ),
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),

              // Tab content
              Container(
                height: 140,
                color: Colors.black,
                child: _buildTabPanel(),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
