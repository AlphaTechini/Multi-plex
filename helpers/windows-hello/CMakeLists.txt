cmake_minimum_required(VERSION 3.20)
project(mpx-hello LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
# Static C/C++ runtime (/MT): no VC++ redistributable needed on the user's machine.
set(CMAKE_MSVC_RUNTIME_LIBRARY "MultiThreaded$<$<CONFIG:Debug>:Debug>")

add_executable(mpx-hello mpx-hello.cpp)
target_compile_options(mpx-hello PRIVATE /EHsc /W4 /permissive-)
target_compile_definitions(mpx-hello PRIVATE WIN32_LEAN_AND_MEAN NOMINMAX UNICODE _UNICODE)
target_link_libraries(mpx-hello PRIVATE windowsapp user32)
set_target_properties(mpx-hello PROPERTIES OUTPUT_NAME mpx-hello)
