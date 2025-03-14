const std = @import("std");
const aisap = @import("aisap");
const AppImage = aisap;
//const AppImage = aisap.AppImage;

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    const allocator = gpa.allocator();

    var args = std.process.args();

    const stdout_file = std.io.getStdOut();
    const stdout = stdout_file.writer();

    _ = args.next();

    const target = args.next() orelse {
        std.debug.print("Error: no AppImage provided\n", .{});
        return;
    };

    const cwd = std.fs.cwd();
    const file = try cwd.openFile(target, .{});
    defer file.close();

    var appimage = try AppImage.open(
        allocator,
        file,
    );
    defer appimage.close();

    try stdout.print("AppImage info:\n", .{});
    try stdout.print("  kind:    {}\n", .{appimage.kind});
    try stdout.print("  offset:  {d}\n", .{appimage.sqfs.opts.offset});
    //  try stdout.print("  inode count: {d}\n", .{sqfs.super_block.inode_count});
}
